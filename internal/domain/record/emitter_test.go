package record_test

// El emisor: quién pone el sobre y qué pasa con los hechos que ocurren ANTES de
// que el intento tenga identidad.
//
// El búfer no es una optimización. La tira vive en
// `events/<deployment_id>/<execution_id>.jsonl`, así que no hay dónde escribir
// hasta que el resolutor deriva el `deployment_id` — y hay un hecho que ocurre
// antes, `stale_clone_used`, que lo observa el clonador siete eslabones más
// arriba (spec 18 §5.4).

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

var instanteDelEmisor = time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)

// sinkEspia recuerda lo que se le entregó, en el orden en que llegó.
type sinkEspia struct {
	stream   record.EventStream
	events   []record.Event
	llamadas int
}

var _ record.EventSink = (*sinkEspia)(nil)

func (s *sinkEspia) Append(_ *context.Context, stream record.EventStream, events []record.Event) error {
	s.stream = stream
	s.events = append(s.events, events...)
	s.llamadas++
	return nil
}

// idsFijos compone identificadores deterministas. La entropía es infraestructura
// —igual que el reloj— y por eso el dominio la pide por un puerto.
type idsFijos struct{}

var _ record.EventIDFactory = idsFijos{}

func (idsFijos) New(at time.Time) (record.EventID, error) {
	return record.NewEventID(at, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
}

func tiraDePrueba(t *testing.T) record.EventStream {
	t.Helper()
	content, err := deployment.ParseContentID(
		"cnt-v1:" + "1111111111111111111111111111111111111111111111111111111111111111")
	require.NoError(t, err)
	id, err := deployment.DeploymentIDOf(content, deployment.DeploymentID{})
	require.NoError(t, err)
	return record.EventStream{Deployment: id, ExecutionID: "exec-1"}
}

func emisorDePrueba() (*record.Emitter, *sinkEspia) {
	sink := &sinkEspia{}
	return record.NewEmitter(shared.NewFixedClock(instanteDelEmisor), idsFijos{}, sink), sink
}

// Lo observado antes de abrir la tira no se pierde: se retiene y se escribe al
// abrirla, conservando su POSICIÓN. `seq` es el orden en que las cosas pasaron,
// no el orden en que se pudieron escribir.
func TestEmitter_LoRetenidoSeEscribeAlAbrirYConservaSuPosicion(t *testing.T) {
	ctx := context.Background()
	emitter, sink := emisorDePrueba()

	require.NoError(t, emitter.Emit(&ctx, record.StaleCloneUsed{
		Source: "https://vex.test/acme/pipelinecode", AgeHours: 30}))
	assert.Empty(t, sink.events, "sin tira abierta no hay dónde escribir")

	tira := tiraDePrueba(t)
	require.NoError(t, emitter.Open(&ctx, tira, deployment.FirstAttempt()))

	require.Len(t, sink.events, 1)
	assert.Equal(t, record.TypeStaleCloneUsed, sink.events[0].Type())
	assert.Equal(t, uint64(1), sink.events[0].Seq().Position(),
		"el clon viejo se usó ANTES de que el intento tuviera identidad, y el pliegue tiene que poder decirlo")
	assert.Equal(t, tira, sink.stream)

	require.NoError(t, emitter.Emit(&ctx, record.AttemptStarted{Deployment: tira.Deployment}))

	require.Len(t, sink.events, 2)
	assert.Equal(t, uint64(2), sink.events[1].Seq().Position())
	assert.Equal(t, 2, sink.llamadas, "con la tira abierta cada hecho se escribe cuando ocurre")
}

// Sin nada retenido, abrir no escribe: no tener hechos que esperar es lo normal.
func TestEmitter_AbrirSinNadaRetenidoNoEscribe(t *testing.T) {
	ctx := context.Background()
	emitter, sink := emisorDePrueba()

	require.NoError(t, emitter.Open(&ctx, tiraDePrueba(t), deployment.FirstAttempt()))

	assert.Zero(t, sink.llamadas)
}

// Abrir dos veces no es reabrir: significaría dos despliegues escribiendo en la
// misma numeración de `seq`.
func TestEmitter_AbrirDosVecesEsUnError(t *testing.T) {
	ctx := context.Background()
	emitter, _ := emisorDePrueba()
	tira := tiraDePrueba(t)

	require.NoError(t, emitter.Open(&ctx, tira, deployment.FirstAttempt()))
	require.Error(t, emitter.Open(&ctx, tira, deployment.FirstAttempt()))
}

// Una tira a medias no vale: sin despliegue no hay dónde vivir, y sin ejecución
// no se distingue de la de al lado.
func TestEmitter_UnaTiraAMediasNoSeAbre(t *testing.T) {
	ctx := context.Background()
	tira := tiraDePrueba(t)

	for _, caso := range []struct {
		nombre string
		tira   record.EventStream
	}{
		{"sin despliegue", record.EventStream{ExecutionID: "exec-1"}},
		{"sin ejecución", record.EventStream{Deployment: tira.Deployment}},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			emitter, _ := emisorDePrueba()
			require.Error(t, emitter.Open(&ctx, caso.tira, deployment.FirstAttempt()))
		})
	}

	t.Run("sin intento", func(t *testing.T) {
		emitter, _ := emisorDePrueba()
		require.Error(t, emitter.Open(&ctx, tira, deployment.Attempt{}))
	})
}

// Un hecho inválido es un error DEL EMISOR, no un evento degradado que se
// escribe igual: un hecho roto contamina el pliegue de los que vengan detrás.
func TestEmitter_UnHechoInvalidoNoSeEscribe(t *testing.T) {
	ctx := context.Background()
	emitter, sink := emisorDePrueba()

	require.NoError(t, emitter.Open(&ctx, tiraDePrueba(t), deployment.FirstAttempt()))

	err := emitter.Emit(&ctx, record.StaleCloneUsed{Source: "", AgeHours: 1})

	require.Error(t, err)
	assert.Empty(t, sink.events)
}
