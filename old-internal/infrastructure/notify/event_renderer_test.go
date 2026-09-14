package notify_test

// La inversión de R-7: las líneas para humanos se DERIVAN de los hechos.
//
// Hasta la spec 19 el log era la fuente y no quedaba nada —`Emit` producía texto
// libre y `SupabaseLogObserver` lo tiraba bajo backpressure—. Aquí el registro es
// la fuente y la narrativa es una proyección suya, que son dos cosas que cambian
// por razones distintas: la narrativa cuando se quiere leer mejor, los hechos
// cuando cambia lo que el motor hace.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	domRecord "github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/state"
	"github.com/jairoprogramador/vex-engine/old-internal/infrastructure/notify"
)

var instante = time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC)

func TestEventRenderer_LasLineasSalenDeLosHechos(t *testing.T) {
	casos := []struct {
		nombre   string
		carga    domRecord.Payload
		esperada string
	}{
		{
			nombre:   "un step que empieza",
			carga:    domRecord.StepStarted{StepID: "01-test"},
			esperada: "Step 01-test en ejecución",
		},
		{
			nombre: "un step que termina bien, con su duración",
			carga: domRecord.StepFinished{
				StepID: "01-test", Status: command.StepSuccess,
				Reason: command.ReasonNoRecord, Duration: 1500 * time.Millisecond},
			esperada: "Step 01-test ejecutado correctamente (1.5s)",
		},
		{
			nombre: "un step saltado dice por qué",
			carga: domRecord.StepFinished{
				StepID: "02-supply", Status: command.StepSkipped,
				Reason: command.ReasonNoCommands},
			esperada: "Step 02-supply saltado: no_commands",
		},
		{
			nombre: "un step que falla",
			carga: domRecord.StepFinished{
				StepID: "01-test", Status: command.StepFailure,
				Duration: 2 * time.Second},
			esperada: "Step 01-test ejecución fallida (2s)",
		},
		{
			nombre:   "un comando que empieza",
			carga:    domRecord.CommandStarted{StepID: "01-test", CommandName: "build"},
			esperada: "Comando build en ejecución",
		},
		{
			// El exit code entra en la línea, y hasta la spec 19 no estaba en
			// ninguna: el texto decía «ejecución fallida» y el número vivía dentro
			// del error, tres capas más arriba.
			nombre: "un comando que falla dice con qué código",
			carga: domRecord.CommandFinished{
				StepID: "01-test", CommandName: "build",
				Status: command.CommandFailure, ExitCode: 7},
			esperada: "Comando build ejecución fallida (exit code 7)",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			espia := &observadorEspia{}
			renderer := notify.NewEventRenderer(&sinkEspia{})
			renderer.Observe(espia)

			require.NoError(t, anexar(t, renderer, caso.carga))
			assert.Equal(t, []string{caso.esperada}, espia.lineas)
		})
	}
}

// EL CASO QUE MÁS GANA CON LA INVERSIÓN. La frase «ejecutado el X por Y» la
// componía el handler que decide, con datos que se perdían al formatearlos.
// Ahora esos datos son la EVIDENCIA —la referencia entera al registro— y la
// frase es una proyección suya: la pregunta que el motor promete responder deja
// de vivir sólo en una línea de log descartable.
func TestEventRenderer_UnStepRevividoDiceCuandoSeProboPorUltimaVez(t *testing.T) {
	espia := &observadorEspia{}
	renderer := notify.NewEventRenderer(&sinkEspia{})
	renderer.Observe(espia)

	require.NoError(t, anexar(t, renderer, domRecord.StepFinished{
		StepID:    "01-test",
		Status:    command.StepCached,
		FromCache: true,
		Reason:    command.ReasonUpToDate,
		Evidence: domRecord.EvidenceRef{
			ExecutionID: "exec-anterior",
			At:          instante.Add(-24 * time.Hour),
			StateKey:    clave(t),
			RecordID:    registro(t),
		},
	}))

	require.Len(t, espia.lineas, 1)
	assert.Contains(t, espia.lineas[0], "Step 01-test sin cambios")
	assert.Contains(t, espia.lineas[0], "2026-08-08T10:00:00Z")
	assert.Contains(t, espia.lineas[0], "exec-anterior")
}

// No todo hecho produce línea, y es deliberado: `parameter_resolved` es uno por
// parámetro y no tiene nada que contarle a un humano en mitad de una ejecución.
func TestEventRenderer_NoTodoHechoSeNarra(t *testing.T) {
	espia := &observadorEspia{}
	renderer := notify.NewEventRenderer(&sinkEspia{})
	renderer.Observe(espia)

	require.NoError(t, anexar(t, renderer, domRecord.ParameterResolved{
		Name: "acr_name", Source: command.OriginRuntime, Digest: "sha256:abc"}))
	require.NoError(t, anexar(t, renderer, domRecord.CommandFinished{
		StepID: "01-test", CommandName: "build", Status: command.CommandSuccess}))

	assert.Empty(t, espia.lineas)
}

// REGISTRAR ES INCONDICIONAL, NARRAR NO (§5.6). Sin observador el renderizador
// es un paso a través, y los hechos llegan al archivo igual: el motor escribe
// siempre en su área de trabajo, esté o no mirando alguien.
func TestEventRenderer_SinObservadorLosHechosSeEscribenIgual(t *testing.T) {
	sink := &sinkEspia{}
	renderer := notify.NewEventRenderer(sink)

	require.NoError(t, anexar(t, renderer, domRecord.StepStarted{StepID: "01-test"}))

	assert.Len(t, sink.recibidos, 1)
}

// Y el error del sink SUBE: el renderizador decora, no absorbe. Si el archivo no
// se pudo escribir, quien lo emitió tiene que enterarse.
func TestEventRenderer_ElErrorDelSinkSube(t *testing.T) {
	errEscritura := errors.New("disco lleno")
	renderer := notify.NewEventRenderer(&sinkEspia{err: errEscritura})
	espia := &observadorEspia{}
	renderer.Observe(espia)

	err := anexar(t, renderer, domRecord.StepStarted{StepID: "01-test"})

	require.ErrorIs(t, err, errEscritura)
	assert.NotEmpty(t, espia.lineas,
		"la línea es la proyección de un hecho OBSERVADO: que llegara al disco es otra cosa")
}

// ── Dobles ──────────────────────────────────────────────────────────────────

func anexar(t *testing.T, sink domRecord.EventSink, carga domRecord.Payload) error {
	t.Helper()

	id, err := domRecord.NewEventID(instante, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	require.NoError(t, err)
	evento, err := domRecord.NewEvent(
		id, domRecord.FirstSeq(), instante, deployment.FirstAttempt(), carga)
	require.NoError(t, err)

	ctx := context.Background()
	return sink.Append(&ctx, domRecord.EventStream{
		Deployment:  despliegue(t),
		ExecutionID: "exec-1",
	}, []domRecord.Event{evento})
}

func despliegue(t *testing.T) deployment.DeploymentID {
	t.Helper()
	id, err := deployment.ParseDeploymentID(
		deployment.DeploymentIDVersion + ":" + strings.Repeat("ab", 32))
	require.NoError(t, err)
	return id
}

func clave(t *testing.T) state.Key {
	t.Helper()
	scope, err := state.NewEnvironmentScope("sand")
	require.NoError(t, err)
	key, err := state.NewKey("https://vex.test/acme/demo-app", scope, "01-test")
	require.NoError(t, err)
	return key
}

func registro(t *testing.T) state.RecordID {
	t.Helper()
	id, err := state.NewRecordID(instante.Add(-24*time.Hour),
		[]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	require.NoError(t, err)
	return id
}

type observadorEspia struct{ lineas []string }

func (o *observadorEspia) Notify(_ string, line string) { o.lineas = append(o.lineas, line) }

type sinkEspia struct {
	recibidos []domRecord.Event
	err       error
}

var _ domRecord.EventSink = (*sinkEspia)(nil)

func (s *sinkEspia) Append(
	_ *context.Context, _ domRecord.EventStream, events []domRecord.Event) error {

	s.recibidos = append(s.recibidos, events...)
	return s.err
}
