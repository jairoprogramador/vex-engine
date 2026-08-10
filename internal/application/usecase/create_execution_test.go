package usecase_test

// Quién cierra el ciclo de vida (spec 07 §5.2).
//
// El use case es la ÚNICA capa que ve el éxito y el fallo de la cadena, así
// que es la que los registra: «un solo dueño por hecho». Hasta ahora devolvía
// `queued` por los dos caminos y era la CLI —tres capas afuera— quien deducía
// el estado terminal a partir de un `error`.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

var instanteFijo = time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)

// pipelineDePrueba sustituye a la cadena entera: lo que se prueba aquí es qué
// hace el use case con lo que la cadena devuelva, no la cadena.
type pipelineDePrueba struct {
	err error
}

func (p pipelineDePrueba) Execute(*command.ExecutionContext) error {
	return p.err
}

type emisorMudo struct{}

func (emisorMudo) Notify(string, string) {}

// emisorDeHechos es un emisor SIN TIRA ABIERTA, que es exactamente el estado en
// el que esta capa lo encuentra cuando la ejecución no llega al resolutor de
// despliegue: retiene lo que se le entrega y no toca ningún sink.
//
// Es lo que hace que estos casos sigan midiendo lo suyo —qué estado terminal
// publica el agregado— sin montar medio registro. Que `attempt_finished` llegue
// al archivo es una propiedad del cableado completo y se mide en integración.
func emisorDeHechos(t *testing.T) *record.Emitter {
	t.Helper()
	return record.NewEmitter(shared.NewFixedClock(instanteFijo), nil, nil)
}

func requestValido() dto.RequestInput {
	return dto.RequestInput{
		SchemaVersion: 1,
		Project: dto.ProjectInput{
			Id: "11111111-1111-1111-1111-111111111111", Name: "demo-app",
			Team: "plataforma", Org: "acme",
			Url: "https://vex.test/acme/demo-app", Ref: "main",
		},
		Pipeline:  dto.PipelineInput{Url: "https://vex.test/acme/pipelinecode", Ref: "main"},
		Execution: dto.ExecutionInput{Step: "supply", Environment: "sand"},
	}
}

func ejecutar(t *testing.T, ctx context.Context, pipeline command.Executable) (usecase.CreateExecutionOutput, error) {
	t.Helper()
	uc := usecase.NewCreateExecutionUseCase(
		pipeline, nil, nil, shared.NewFixedClock(instanteFijo), emisorDeHechos(t)).
		WithObservers(emisorMudo{}, nil)
	return uc.Execute(ctx, requestValido(), "exec-1")
}

func TestCreateExecution_UnaPipelineExitosaTerminaEnSucceeded(t *testing.T) {
	output, err := ejecutar(t, context.Background(), pipelineDePrueba{})

	require.NoError(t, err)
	// Que el estado sea `succeeded` y no `queued` dice además que MarkRunning se
	// invocó antes de la cadena: terminar sin haber empezado es una transición
	// ilegal, y el agregado la habría rechazado dejando el estado como estaba.
	assert.Equal(t, "succeeded", output.Status)
	require.NotNil(t, output.ExitCode)
	assert.Equal(t, 0, *output.ExitCode)
	require.NotNil(t, output.FinishedAt)
	assert.Equal(t, instanteFijo, *output.FinishedAt)
}

func TestCreateExecution_ElExitCodeDelComandoQueFalloLlegaAlAgregado(t *testing.T) {
	pipeline := pipelineDePrueba{
		err: command.NewCommandFailedError("mvn test", 3, "BUILD FAILURE"),
	}

	output, err := ejecutar(t, context.Background(), pipeline)

	require.Error(t, err)
	assert.Equal(t, "failed", output.Status)
	require.NotNil(t, output.ExitCode)
	assert.Equal(t, 3, *output.ExitCode)
}

// Un fallo que no es de un comando —un clone, una validación del
// pipelinecode— no tiene exit code propio.
func TestCreateExecution_UnFalloSinComandoUsaElExitCodeGenerico(t *testing.T) {
	pipeline := pipelineDePrueba{err: errors.New("clonar el proyecto: repositorio no encontrado")}

	output, err := ejecutar(t, context.Background(), pipeline)

	require.Error(t, err)
	assert.Equal(t, "failed", output.Status)
	require.NotNil(t, output.ExitCode)
	assert.Equal(t, command.DefaultFailureExitCode, *output.ExitCode)
}

// Cancelar mata el comando en curso, así que la cadena devuelve un error como
// cualquier otro fallo. Lo que distingue los dos casos es el contexto: una
// decisión no es una desgracia, y el registro tiene que poder decirlo.
func TestCreateExecution_UnContextoCanceladoNoEsUnFallo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pipeline := pipelineDePrueba{err: errors.New("context canceled")}

	output, err := ejecutar(t, ctx, pipeline)

	require.Error(t, err, "el error de la cadena sigue propagándose")
	assert.Equal(t, "canceled", output.Status)
	assert.Nil(t, output.ExitCode, "una cancelación no lleva exit code")
}
