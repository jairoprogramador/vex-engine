package sync_test

// Las propiedades de §7 que se observan sin disco: cuántas veces se empuja,
// cuándo avanza el `ack` y qué pasa cuando el destino no responde.
//
// El adaptador de archivo tiene sus propias pruebas —la idempotencia contra un
// destino real— y el harness de integración prueba el conjunto. Aquí lo que se
// fija es la POLÍTICA, que es lo único del empuje que vive en el dominio.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/step"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/sync"
)

// ── Dobles ──────────────────────────────────────────────────────────────────

// sinkDePrueba cuenta empujes y devuelve lo que se le diga. El `confirma` es lo
// que el destino dice haber recibido, que es lo único con lo que el `ack` puede
// avanzar.
type sinkDePrueba struct {
	llamadas []sync.Batch
	confirma uint64
	fallar   error
	// fallarHasta hace que los primeros N empujes fallen y el resto funcione. Es
	// lo que permite observar la recuperación sin tocar disco.
	fallarHasta int
}

func (s *sinkDePrueba) Push(_ *context.Context, batch sync.Batch) (record.Seq, error) {
	s.llamadas = append(s.llamadas, batch)
	if len(s.llamadas) <= s.fallarHasta {
		return record.Seq{}, errors.New("el destino no responde")
	}
	if s.fallar != nil {
		return record.Seq{}, s.fallar
	}
	seq, err := record.NewSeq(s.confirma)
	if err != nil {
		return record.Seq{}, nil
	}
	return seq, nil
}

func (s *sinkDePrueba) Destination() string { return "local:/mnt/vex-state" }

type ackDePrueba struct {
	seq       record.Seq
	guardados []record.Seq
	leerErr   error
}

func (a *ackDePrueba) Last(_ *context.Context, _ record.EventStream) (record.Seq, error) {
	if a.leerErr != nil {
		return record.Seq{}, a.leerErr
	}
	return a.seq, nil
}

func (a *ackDePrueba) Save(_ *context.Context, _ record.EventStream, confirmed record.Seq) error {
	a.guardados = append(a.guardados, confirmed)
	a.seq = confirmed
	return nil
}

type hechosDePrueba struct {
	fallos []string
}

func (h *hechosDePrueba) SyncFailed(_ *context.Context, destination, cause string) error {
	h.fallos = append(h.fallos, destination+" | "+cause)
	return nil
}

// dormirDePrueba anota las esperas sin esperarlas. Es la razón por la que la
// espera es un puerto: probar una política de reintento no puede costar lo que
// la política dice.
type dormirDePrueba struct {
	esperas []time.Duration
}

func (d *dormirDePrueba) Sleep(_ *context.Context, duration time.Duration) error {
	d.esperas = append(d.esperas, duration)
	return nil
}

// ── Fixture ─────────────────────────────────────────────────────────────────

func tiraDePrueba(t *testing.T) record.EventStream {
	t.Helper()

	id, err := deployment.ParseDeploymentID(
		deployment.DeploymentIDVersion + ":" + strings.Repeat("ab", 32))
	require.NoError(t, err)
	return record.EventStream{Deployment: id, ExecutionID: "exec-1"}
}

func contenidoDePrueba(t *testing.T) deployment.Content {
	t.Helper()

	huella := func(version, digito string) fingerprint.Fingerprint {
		f, err := fingerprint.Parse(version + ":" + strings.Repeat(digito, 32))
		require.NoError(t, err)
		return f
	}

	subject, err := deployment.NewSubject("https://vex.test/acme/demo-app")
	require.NoError(t, err)
	operation, err := deployment.NewOperation("deploy")
	require.NoError(t, err)
	destination, err := deployment.NewDestination("sand")
	require.NoError(t, err)
	source, err := deployment.NewSource(
		huella(fingerprint.Version, "33"), huella(fingerprint.Version, "44"))
	require.NoError(t, err)
	format, err := deployment.NewFormat(2, true)
	require.NoError(t, err)

	stepContent, err := deployment.NewStepContent(
		"01-test", step.NoStepConfig(), huella(fingerprint.DeclarationVersion, "11"), nil)
	require.NoError(t, err)

	content, err := deployment.NewContent(
		subject, operation, destination, source, format, []deployment.StepContent{stepContent})
	require.NoError(t, err)
	return content
}

type banco struct {
	sink   *sinkDePrueba
	acks   *ackDePrueba
	hechos *hechosDePrueba
	dormir *dormirDePrueba
	sut    *sync.Synchronizer
	ctx    *context.Context
}

func montar(t *testing.T, sink *sinkDePrueba) *banco {
	t.Helper()

	ctx := context.Background()
	b := &banco{
		sink:   sink,
		acks:   &ackDePrueba{},
		hechos: &hechosDePrueba{},
		dormir: &dormirDePrueba{},
		ctx:    &ctx,
	}
	b.sut = sync.NewSynchronizer(
		b.sink, b.acks, b.hechos, sync.DefaultRetryPolicy(), b.dormir)
	return b
}

func (b *banco) enlazar(t *testing.T) {
	t.Helper()
	b.sut.Bind(b.ctx, tiraDePrueba(t), contenidoDePrueba(t), deployment.ObjectMetadata{})
}

// ── Casos ───────────────────────────────────────────────────────────────────

func TestSynchronizer_SinEnlaceNoEmpujaYNoEsUnFallo(t *testing.T) {
	// Una ejecución que muere antes del resolutor no tiene `deployment_id`, así
	// que no hay tira a la que pertenezcan sus hechos. No empujar es correcto, y
	// no es un hueco que explicar (spec 18 §5.1).
	b := montar(t, &sinkDePrueba{confirma: 5})

	b.sut.Push(b.ctx)

	assert.Empty(t, b.sink.llamadas)
	assert.Empty(t, b.hechos.fallos)
}

func TestSynchronizer_ElAckAvanzaConLoQueElDestinoConfirmaYNoAntes(t *testing.T) {
	b := montar(t, &sinkDePrueba{confirma: 7})
	b.enlazar(t)

	b.sut.Push(b.ctx)

	require.Len(t, b.acks.guardados, 1)
	assert.Equal(t, uint64(7), b.acks.guardados[0].Position(),
		"lo que se guarda es lo que el destino dijo, no lo que el llamador creyó mandar")
}

func TestSynchronizer_ConUnDestinoQueFallaElAckNoAvanza(t *testing.T) {
	// La propiedad que impide el hueco: un `ack` optimista daría por confirmado
	// algo que el destino no tiene, y el empuje siguiente no lo volvería a mandar.
	b := montar(t, &sinkDePrueba{fallar: errors.New("disco lleno")})
	b.enlazar(t)

	b.sut.Push(b.ctx)

	assert.Empty(t, b.acks.guardados, "el ack no avanzó")
	require.Len(t, b.hechos.fallos, 1, "y el hueco quedó explicado")
	assert.Contains(t, b.hechos.fallos[0], "local:/mnt/vex-state")
	assert.Contains(t, b.hechos.fallos[0], "disco lleno")
}

func TestSynchronizer_UnFalloAgotaLosReintentosConEsperaCreciente(t *testing.T) {
	b := montar(t, &sinkDePrueba{fallar: errors.New("timeout")})
	b.enlazar(t)

	b.sut.Push(b.ctx)

	assert.Len(t, b.sink.llamadas, 3, "la política por defecto son tres intentos")
	assert.Equal(t, []time.Duration{200 * time.Millisecond, 400 * time.Millisecond},
		b.dormir.esperas, "no se espera antes del primero, y la espera dobla")
	assert.Len(t, b.hechos.fallos, 1, "un sync_failed por empuje, no uno por intento")
}

func TestSynchronizer_ElSiguienteEmpujeRecogeLoPendienteSinLogicaExtra(t *testing.T) {
	// La recuperación de §7: el primer empuje falla, el segundo funciona, y como
	// el `ack` no avanzó la selección del segundo arranca donde arrancaba la del
	// primero. No hay código de recuperación; hay ausencia de checkpoint.
	sink := &sinkDePrueba{confirma: 9, fallarHasta: 3}
	b := montar(t, sink)
	b.enlazar(t)

	b.sut.Push(b.ctx) // agota los tres intentos y falla
	b.sut.Push(b.ctx) // el del step siguiente

	require.Len(t, sink.llamadas, 4)
	for _, lote := range sink.llamadas {
		assert.True(t, lote.From().IsZero(),
			"todos los intentos parten de la misma posición: el ack no se movió")
	}
	require.Len(t, b.acks.guardados, 1)
	assert.Equal(t, uint64(9), b.acks.guardados[0].Position())
}

func TestSynchronizer_PerderElAckDegradaAReenvioTotal(t *testing.T) {
	// Es lo que lo hace una optimización y no un checkpoint: si no se puede leer,
	// se manda la tira entera, que es idempotente.
	b := montar(t, &sinkDePrueba{confirma: 4})
	b.acks.seq = mustSeq(t, 3)
	b.acks.leerErr = errors.New("el archivo no está")
	b.enlazar(t)

	b.sut.Push(b.ctx)

	require.Len(t, b.sink.llamadas, 1)
	assert.True(t, b.sink.llamadas[0].From().IsZero(),
		"ante la duda se reenvía desde el principio")
	assert.Empty(t, b.hechos.fallos, "perder el ack no es un hueco que explicar")
}

func TestSynchronizer_ElAckNoRetrocede(t *testing.T) {
	// Un destino que responde una posición menor que la que ya constaba no puede
	// hacer retroceder el puntero: costaría ancho de banda y taparía el síntoma.
	b := montar(t, &sinkDePrueba{confirma: 2})
	b.acks.seq = mustSeq(t, 6)
	b.enlazar(t)

	b.sut.Push(b.ctx)

	assert.Empty(t, b.acks.guardados)
	assert.Equal(t, uint64(6), b.acks.seq.Position())
}

func TestSynchronizer_ElLoteLlevaLaSeleccionYLaIntencionEntera(t *testing.T) {
	b := montar(t, &sinkDePrueba{confirma: 3})
	b.acks.seq = mustSeq(t, 2)
	b.enlazar(t)

	b.sut.Push(b.ctx)

	require.Len(t, b.sink.llamadas, 1)
	lote := b.sink.llamadas[0]
	assert.Equal(t, uint64(2), lote.From().Position())
	assert.Equal(t, "exec-1", lote.Stream().ExecutionID)
	assert.Equal(t, contenidoDePrueba(t).Canonical(), lote.Content().Canonical(),
		"lo que se empuja de un objeto es su MATERIAL, no su hash")
}

func TestSynchronizer_ElReceptorNilNoEmpujaYNoRevienta(t *testing.T) {
	// Es lo que permite montar el motor sin destino —o una prueba de la capa de
	// aplicación— sin sembrar comprobaciones en los dos llamadores.
	var ausente *sync.Synchronizer
	ctx := context.Background()

	assert.NotPanics(t, func() {
		ausente.Bind(&ctx, tiraDePrueba(t), contenidoDePrueba(t), deployment.ObjectMetadata{})
		ausente.Push(&ctx)
	})
}

func mustSeq(t *testing.T, position uint64) record.Seq {
	t.Helper()
	seq, err := record.NewSeq(position)
	require.NoError(t, err)
	return seq
}
