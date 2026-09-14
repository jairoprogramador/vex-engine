package sync

import (
	"context"
	stdsync "sync"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
)

// Synchronizer es el servicio de dominio que empuja lo registrado al destino.
//
// # Cuándo empuja, y por qué no es un detalle (§4, D2.5)
//
//	al terminar cada step:      Push  (lo pendiente desde el `ack`)
//	al cerrar (éxito o fallo):  Push  (el MISMO código)
//
// Por evento penalizaría la ejecución con red constante —decenas de round-trips
// dentro de un `deploy`— y un destino lento retrasaría el despliegue, que es
// inaceptable: el registro no puede ralentizar lo que observa. Sólo al final
// concentra todo el riesgo en el peor momento posible: si el proceso muere justo
// antes, se pierde el registro COMPLETO, y precisamente en el escenario de fallo
// que más interesa conservar.
//
// Por step acota la pérdida al step en curso con un coste proporcional al número
// de steps (4–5, no 50), y lo único que pide a cambio es que el empuje sea
// idempotente — que lo es por construcción (§5.2).
//
// # Es de UNA ejecución, como el emisor
//
// Se cablea una vez y se enlaza al intento cuando ese intento tiene identidad
// (`Bind`). Un `Push` antes del enlace no es un error: es un intento que murió
// antes del resolutor, y no hay tira que empujar porque no hay `deployment_id`
// al que pertenezca (spec 18 §5.1).
//
// # El receptor nil es «no hay a dónde empujar»
//
// `Bind` y `Push` toleran el receptor nil a propósito: es lo que permite montar
// el motor sin sincronización —una prueba de la capa de aplicación, un cableado
// que todavía no tiene destino— sin sembrar comprobaciones en los dos llamadores.
// Un sincronizador ausente no es un fallo: es la ausencia del empuje, que es
// exactamente lo que el motor hacía hasta esta spec.
//
// # Ninguno de sus métodos devuelve error, y ésa es la regla
//
// **El registro nunca puede hacer fallar lo que observa** (§5.3'). Lo que aquí
// sale mal se cuenta como `sync_failed` y se devuelve el control. La única
// excepción del sistema es `attempt_finished` (spec 19), y la asimetría está
// razonada: allí lo que se pierde es la historia entera del intento; aquí, una
// copia de algo que sigue íntegro en el área de trabajo.
type Synchronizer struct {
	sink    Sink
	acks    AckStore
	facts   FactSink
	policy  RetryPolicy
	sleeper Sleeper

	// El motor es de un solo hilo, pero quien enlaza (la cadena de pipeline) y
	// quien empuja al cerrar (el caso de uso) son dos momentos distintos del
	// proceso. Cuesta un mutex lo mismo que en el emisor, y por lo mismo.
	mu       stdsync.Mutex
	stream   record.EventStream
	content  deployment.Content
	metadata deployment.ObjectMetadata
	bound    bool
}

func NewSynchronizer(
	sink Sink,
	acks AckStore,
	facts FactSink,
	policy RetryPolicy,
	sleeper Sleeper) *Synchronizer {

	return &Synchronizer{
		sink:    sink,
		acks:    acks,
		facts:   facts,
		policy:  policy,
		sleeper: sleeper,
	}
}

// Bind dice a qué intento pertenece lo que se va a empujar.
//
// Lo llama la cadena de pipeline en cuanto la identidad existe y ANTES del
// primer step, no en el primer empuje: un intento que resuelve su despliegue y
// falla dentro del step 1 no llega a ningún `Push` del bucle, y el cierre tiene
// que poder empujar su tira igual. Sin esto, el caso de fallo —el que más
// importa conservar— sería justamente el que se pierde.
//
// Un enlace que no se puede componer deja al sincronizador MUDO y se registra
// como `sync_failed`: es un hueco que hay que explicar, no un despliegue que
// haya que detener.
func (s *Synchronizer) Bind(
	ctx *context.Context,
	stream record.EventStream,
	content deployment.Content,
	metadata deployment.ObjectMetadata) {

	if s == nil {
		return
	}

	batch, err := NewBatch(stream, record.Seq{}, content, metadata)
	if err != nil {
		s.report(ctx, err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.stream = batch.Stream()
	s.content = batch.Content()
	s.metadata = batch.Metadata()
	s.bound = true
}

// Push empuja lo pendiente desde el `ack`.
//
// Es el mismo código al terminar un step y al cerrar el intento: no hay una vía
// «normal» y otra «final», y no tener dos es lo que hace que la recuperación
// salga gratis. Si un empuje falla, el siguiente step recoge lo pendiente sin
// lógica extra — el `ack` no avanzó, así que la selección siguiente lo incluye.
func (s *Synchronizer) Push(ctx *context.Context) {
	if s == nil {
		return
	}

	stream, content, metadata, ok := s.binding()
	if !ok {
		// El intento no llegó a tener identidad. No hay tira que empujar, y eso
		// no es un fallo de sincronización: es una ejecución que murió antes del
		// resolutor (spec 18 §5.1).
		return
	}

	confirmadoAntes, err := s.acks.Last(ctx, stream)
	if err != nil {
		// Perder el `ack` degrada a reenvío TOTAL, nunca a hueco: se sigue con la
		// posición cero, que es exactamente lo que su ausencia significa.
		confirmadoAntes = record.Seq{}
	}

	batch, err := NewBatch(stream, confirmadoAntes, content, metadata)
	if err != nil {
		s.report(ctx, err)
		return
	}

	confirmado, err := s.attempt(ctx, batch)
	if err != nil {
		s.report(ctx, err)
		return
	}

	// El `ack` se avanza SÓLO hacia adelante y SÓLO con lo que el destino
	// confirmó. Un destino que responde una posición menor que la que ya
	// constaba no puede hacer retroceder el puntero: retrocederlo costaría ancho
	// de banda, pero además tapa el síntoma de que el destino se está
	// contradiciendo.
	if confirmado.IsZero() || !confirmadoAntes.Before(confirmado) {
		return
	}
	if err := s.acks.Save(ctx, stream, confirmado); err != nil {
		// No es un `sync_failed`: el registro SÍ llegó al destino. Lo único que
		// se pierde es la optimización, y su pérdida degrada a reenvío total en
		// el empuje siguiente — que es idempotente.
		return
	}
}

// attempt agota la política de reintento.
//
// Acotada y NO bloqueante para el pipeline: un fallo agota los intentos, el
// llamador emite `sync_failed` y se devuelve el control.
func (s *Synchronizer) attempt(ctx *context.Context, batch Batch) (record.Seq, error) {
	var ultimo error
	for intento := 1; intento <= s.policy.Attempts(); intento++ {
		if espera := s.policy.WaitBefore(intento); espera > 0 && s.sleeper != nil {
			if err := s.sleeper.Sleep(ctx, espera); err != nil {
				// El contexto murió durante el backoff: no hay más reintentos que
				// hacer, y el error que explica el hueco es el del destino, no el
				// de la espera.
				break
			}
		}

		confirmado, err := s.sink.Push(ctx, batch)
		if err == nil {
			return confirmado, nil
		}
		ultimo = err
	}
	return record.Seq{}, ultimo
}

// report deja escrito por qué el registro remoto va a tener un hueco.
//
// # Dónde acaba este hecho, que es la decisión incómoda de esta spec
//
// Se anexa a la tira que se estaba empujando, o sea que «el envío falló» queda
// DENTRO de lo que no se envió. Suena a contradicción y no lo es, porque el
// `ack` no avanzó: el empuje siguiente —el del step que viene, o el del cierre—
// arranca desde la misma posición y **se lleva el `sync_failed` con él**. El
// único que no puede llegar al destino es el del ÚLTIMO empuje, y ahí la
// imposibilidad es de la aritmética y no del diseño: si el último empuje falló,
// nada emitido después de él puede llegar por definición.
//
// Lo que §5.5 promete se cumple entero: se escribe en el área de trabajo, que es
// local y por definición alcanzable; si además llega al destino, mejor.
//
// Un fallo AL REGISTRAR el fallo se descarta: no hay una tercera capa a la que
// contárselo, y hacer fallar el pipeline por no poder anotar que una copia no se
// hizo sería exactamente lo que §5.5 prohíbe.
func (s *Synchronizer) report(ctx *context.Context, cause error) {
	if s.facts == nil || cause == nil {
		return
	}
	_ = s.facts.SyncFailed(ctx, s.sink.Destination(), cause.Error())
}

func (s *Synchronizer) binding() (
	record.EventStream, deployment.Content, deployment.ObjectMetadata, bool) {

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stream, s.content, s.metadata, s.bound
}
