package step_test

// Los resolutores de declaraciones (spec 14 §5.5 y §5.3').
//
// Lo que se mide aquí es la diferencia observable de la spec: cuando una fuente
// no produce su valor, el error dice QUÉ FUENTE falló. Hasta ahora el handler de
// variables trataba todo fallo de interpolación como «aún no resoluble» —sin
// distinguir un error real de plantilla de una variable que llegaría más tarde—
// porque nadie había declarado cuál es cuál, así que las dos terminaban en
// «variable no existe».

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domState "github.com/jairoprogramador/vex-engine/internal/domain/state"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

// peticionDePrueba arma la petición de la cadena de step sobre un contexto con
// las variables que se le den ya en el mapa acumulado.
func peticionDePrueba(t *testing.T, acumuladas ...command.Variable) *domStep.StepRequestHandler {
	t.Helper()
	contexto := contextoDePrueba(t)
	for _, variable := range acumuladas {
		contexto.AddAccumulatedVar(variable)
	}
	return domStep.NewStepRequestHandler(contexto, contexto.StepName())
}

func TestStepOutputResolver_LeeDelMapaAcumulado(t *testing.T) {
	declaracion, err := domStep.NewStepOutputDeclaration("artefacto", "01-test", "artifact_name")
	require.NoError(t, err)

	t.Run("la fuente produjo el valor", func(t *testing.T) {
		producida, err := command.NewVariable("artifact_name", "demo-app", command.OriginRuntime)
		require.NoError(t, err)
		request := peticionDePrueba(t, producida)

		valor, err := domStep.NewDeclarationResolvers(sinRegistros{}).
			Resolve(request.Ctx(), request, declaracion)

		require.NoError(t, err)
		assert.Equal(t, "demo-app", valor,
			"el consumidor la llama `artefacto` y el productor no tiene que saberlo")
	})

	t.Run("la fuente no la produjo: el error la nombra", func(t *testing.T) {
		request := peticionDePrueba(t)

		_, err := domStep.NewDeclarationResolvers(sinRegistros{}).
			Resolve(request.Ctx(), request, declaracion)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "artefacto", "qué variable quedó sin valor")
		assert.Contains(t, err.Error(), "01-test", "y a QUIÉN reclamarle")
		assert.Contains(t, err.Error(), "artifact_name")
		assert.NotContains(t, err.Error(), "variable no existe")
	})
}

func TestStateResolver_LeeElRegistroDelAmbitoDeclarado(t *testing.T) {
	declaracion, err := domStep.NewStateDeclaration("db_host", "project", "lb-arn")
	require.NoError(t, err)

	t.Run("la clave está en el registro", func(t *testing.T) {
		request := peticionDePrueba(t)

		valor, err := domStep.NewDeclarationResolvers(registrosCon(t, "lb-arn", "arn:aws:lb")).
			Resolve(request.Ctx(), request, declaracion)

		require.NoError(t, err)
		assert.Equal(t, "arn:aws:lb", valor)
	})

	t.Run("no hay registro todavía", func(t *testing.T) {
		request := peticionDePrueba(t)

		_, err := domStep.NewDeclarationResolvers(sinRegistros{}).
			Resolve(request.Ctx(), request, declaracion)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "lb-arn")
		assert.Contains(t, err.Error(), "project")
	})

	t.Run("el registro existe y no lleva la clave", func(t *testing.T) {
		request := peticionDePrueba(t)

		_, err := domStep.NewDeclarationResolvers(registrosCon(t, "otra-cosa", "x")).
			Resolve(request.Ctx(), request, declaracion)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "lb-arn")
	})
}

// ── Dobles ──────────────────────────────────────────────────────────────────

type sinRegistros struct{}

var _ domState.Records = sinRegistros{}

func (sinRegistros) Get(*context.Context, domState.Key, domState.RecordID) (domState.StepRecord, bool, error) {
	return domState.StepRecord{}, false, nil
}

func (sinRegistros) Last(*context.Context, domState.Key) (domState.StepRecord, bool, error) {
	return domState.StepRecord{}, false, nil
}

func (sinRegistros) Append(*context.Context, domState.Key, domState.StepRecord) error { return nil }

type registrosConUnaVariable struct {
	t     *testing.T
	name  string
	value string
}

var _ domState.Records = registrosConUnaVariable{}

func registrosCon(t *testing.T, name, value string) domState.Records {
	return registrosConUnaVariable{t: t, name: name, value: value}
}

func (registrosConUnaVariable) Get(*context.Context, domState.Key, domState.RecordID) (domState.StepRecord, bool, error) {
	return domState.StepRecord{}, false, nil
}

func (r registrosConUnaVariable) Last(*context.Context, domState.Key) (domState.StepRecord, bool, error) {
	return domState.NewUnattributedRecord([]command.Variable{
		varDePrueba(r.t, r.name, r.value, command.OriginState),
	}), true, nil
}

func (registrosConUnaVariable) Append(*context.Context, domState.Key, domState.StepRecord) error {
	return nil
}
