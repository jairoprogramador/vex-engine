package notify

import (
	"context"
	"fmt"
	"sync"
	"time"

	domCommand "github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	domNotify "github.com/jairoprogramador/vex-engine/old-internal/domain/notify"
	domRecord "github.com/jairoprogramador/vex-engine/old-internal/domain/record"
)

// EventRenderer invierte la dependencia entre el log y el registro (R-7).
//
// # La sub-idea invertida de A9, que es el diseño de la spec 19
//
// Hasta aquí el log era la FUENTE y no quedaba nada: el dominio producía texto
// libre para humanos, `SupabaseLogObserver` lo tiraba bajo backpressure y el
// hecho no existía en ninguna parte. Aquí el registro pasa a ser la fuente y las
// líneas se DERIVAN de él. Son dos cosas que cambian por razones distintas —la
// narrativa cuando se quiere leer mejor, los hechos cuando cambia lo que el
// motor hace— y fundirlas es la razón de que hasta ahora no hubiera registro.
//
// # Es un decorador del sink, no un observador aparte
//
// Se interpone entre el emisor y el archivo en vez de suscribirse a un bus: un
// solo proceso, un solo consumidor, y el orden lo garantiza `seq`. Un bus
// añadiría asincronía donde la escritura tiene que ser síncrona para sobrevivir
// a una muerte dura (§5.3').
//
// Y renderiza ANTES de delegar. Lo que se proyecta es un hecho OBSERVADO; que
// además llegara al disco es una propiedad distinta, y si el disco falla el
// error sube por su cuenta. Al revés, un fallo de escritura dejaría además al
// usuario sin la línea de lo que sí ocurrió.
//
// # Ámbito honesto (§5.5)
//
// Aquí se derivan las líneas de CICLO —step empieza, step termina, comando
// empieza, comando falla— que este cambio retira de los ejecutables. Las 34
// llamadas restantes a `Emit` son diagnóstico puntual dentro de un handler y se
// retiran cuando su hecho equivalente exista; retirarlas todas de golpe sería un
// cambio de superficie de usuario metido dentro de una spec de registro.
type EventRenderer struct {
	sink domRecord.EventSink

	// El observador llega POR EJECUCIÓN —depende de flags que el sink no ve— y el
	// sink se cablea una sola vez al construir el motor. El mutex no es por
	// concurrencia de escritura sino porque quien lo instala y quien lo lee son
	// dos momentos distintos del mismo proceso.
	mu       sync.Mutex
	observer domNotify.LogObserver
}

var _ domRecord.EventSink = (*EventRenderer)(nil)

func NewEventRenderer(sink domRecord.EventSink) *EventRenderer {
	return &EventRenderer{sink: sink}
}

// Observe instala el destino de las líneas de esta ejecución. Sin él, el
// renderizador es un paso a través: **los hechos se escriben igual**, porque
// registrar es incondicional y narrar no (§5.6).
func (r *EventRenderer) Observe(observer domNotify.LogObserver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observer = observer
}

func (r *EventRenderer) Append(
	ctx *context.Context, stream domRecord.EventStream, events []domRecord.Event) error {

	r.render(stream.ExecutionID, events)
	return r.sink.Append(ctx, stream, events)
}

func (r *EventRenderer) render(executionID string, events []domRecord.Event) {
	r.mu.Lock()
	observer := r.observer
	r.mu.Unlock()

	if observer == nil {
		return
	}
	for _, event := range events {
		line, ok := lineOf(event.Payload())
		if !ok {
			continue
		}
		observer.Notify(executionID, line)
	}
}

// lineOf es la proyección: de hecho a frase.
//
// No todos los hechos producen línea, y es deliberado. Los que aquí faltan o no
// tienen nada que contarle a un humano en mitad de una ejecución
// —`parameter_resolved`, uno por parámetro— o ya los cuenta el handler que los
// decide con más contexto del que el hecho lleva: el clon viejo dice su edad, el
// step que revive dice cuándo se ejecutó, el que se salta dice por qué. Duplicar
// esas líneas aquí sería decir dos veces lo mismo con menos precisión.
func lineOf(payload domRecord.Payload) (string, bool) {
	switch carga := payload.(type) {
	case domRecord.StepStarted:
		return "Step " + carga.StepID + " en ejecución", true

	case domRecord.StepFinished:
		return stepFinishedLine(carga)

	case domRecord.CommandStarted:
		return "Comando " + carga.CommandName + " en ejecución", true

	case domRecord.CommandFinished:
		if carga.Status != domCommand.CommandFailure {
			return "", false
		}
		// El exit code entra en la línea, y hasta ahora no estaba en ninguna: el
		// texto decía «ejecución fallida» y el número vivía dentro del error, tres
		// capas más arriba.
		return fmt.Sprintf("Comando %s ejecución fallida (exit code %d)",
			carga.CommandName, carga.ExitCode), true

	case domRecord.SyncFailed:
		// Éste sí produce línea, y no por simetría: un hueco en el registro remoto
		// que sólo se puede descubrir leyendo el archivo local es exactamente lo
		// que la spec 21 §5.5 existe para evitar. Que la línea se DERIVE del hecho
		// —en vez de imprimirla el sincronizador— es lo que garantiza que las dos
		// versiones digan lo mismo.
		return fmt.Sprintf("Registro no sincronizado con %s: %s",
			carga.Destination, carga.Cause), true

	default:
		return "", false
	}
}

// stepFinishedLine es el desenlace del step contado a un humano, y los cuatro
// casos salen del MISMO hecho: es la prueba de que la narrativa se puede derivar
// del registro sin que el dominio tenga que escribirla.
//
// El caso revivido es el que más gana con la inversión: la frase «ejecutado el X
// por Y» la componía el handler que decide, con datos que se perdían al
// formatearlos. Ahora esos datos son la EVIDENCIA —la referencia entera al
// registro— y la frase es una proyección suya. La pregunta que el motor promete
// responder, «¿cuándo se probó esto por última vez?», deja de vivir sólo en una
// línea de log descartable.
func stepFinishedLine(carga domRecord.StepFinished) (string, bool) {
	switch carga.Status {
	case domCommand.StepSkipped:
		return fmt.Sprintf("Step %s saltado: %s", carga.StepID, carga.Reason), true

	case domCommand.StepCached:
		if carga.Evidence.IsZero() {
			return fmt.Sprintf("Step %s sin cambios", carga.StepID), true
		}
		return fmt.Sprintf("Step %s sin cambios (ejecutado el %s por %s)",
			carga.StepID,
			carga.Evidence.At.UTC().Format(time.RFC3339),
			carga.Evidence.ExecutionID), true

	case domCommand.StepSuccess:
		// La duración es el campo con más valor del vocabulario y hasta la spec 19
		// no existía a ningún nivel. La línea la lleva porque es lo primero que se
		// mira al leer una ejecución larga.
		return fmt.Sprintf("Step %s ejecutado correctamente (%s)",
			carga.StepID, humanDuration(carga.Duration)), true

	case domCommand.StepFailure:
		return fmt.Sprintf("Step %s ejecución fallida (%s)",
			carga.StepID, humanDuration(carga.Duration)), true

	default:
		return "", false
	}
}

// humanDuration redondea a milisegundos: la duración de un step es del orden de
// segundos y una cola de nanosegundos sólo estorba al leer.
func humanDuration(duration time.Duration) time.Duration {
	return duration.Round(time.Millisecond)
}
