package pipeline_test

// El invariante estructural del pipelinecode (spec 04 §5.1).
//
// Cada caso de esta tabla es un pipelinecode que HOY —antes de la spec 04— se
// ejecutaba con exit code 0 y un step de menos, o con los steps reordenados. La
// diferencia observable no es que falle: es que el mensaje nombra el directorio
// culpable.

import (
	"context"
	"fmt"
	"testing"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ctxDePrueba y rutaDePrueba son los dos parámetros que la spec 13 añadió a la
// firma del validador. Ninguna de las dos reglas de la spec 04 los mira: la que
// los usa es la del ámbito, que habla de un ARCHIVO del step y por tanto
// necesita saber dónde está el pipelinecode.
func ctxDePrueba() *context.Context {
	c := context.Background()
	return &c
}

const rutaDePrueba = "/pipelinecode"

// configsSinArchivo es el repositorio de `config.yaml` para los casos que no
// hablan de ámbitos: ningún step declara nada, que es legítimo (§5.3).
type configsSinArchivo struct{}

var _ domStep.StepConfigRepository = configsSinArchivo{}

func (configsSinArchivo) Get(*context.Context, string, string) (domStep.StepConfig, error) {
	return domStep.NoStepConfig(), nil
}

func TestPipelineStructureValidator_QueEstructuraSeAcepta(t *testing.T) {
	validador := domPipeline.NewPipelineStructureValidator(configsSinArchivo{})

	casos := []struct {
		nombre    string
		entradas  []string
		valido    bool
		enElError []string
		nota      string
	}{
		{
			nombre:   "cuatro steps de dos dígitos",
			entradas: []string{"01-test", "02-supply", "03-package", "04-deploy"},
			valido:   true,
			nota:     "la forma de los tres templates reales",
		},
		{
			nombre:   "doce steps de dos dígitos",
			entradas: []string{"01-a", "02-b", "03-c", "04-d", "05-e", "06-f", "07-g", "08-h", "09-i", "10-j", "11-k", "12-l"},
			valido:   true,
			nota:     "pasar de nueve steps no rompe nada",
		},
		{
			nombre:   "sin steps",
			entradas: nil,
			valido:   true,
			nota:     "un pipelinecode sin steps no es un error de estructura; el paso pedido no existirá y de eso habla el handler",
		},
		{
			nombre:    "prefijo de un dígito",
			entradas:  []string{"2-supply"},
			valido:    false,
			enElError: []string{"steps/2-supply"},
			nota:      "HOY pasa en silencio: el motor busca steps/02-supply/commands.yaml, no lo encuentra y no ejecuta nada",
		},
		{
			nombre:    "un solo directorio de un dígito entre otros correctos",
			entradas:  []string{"01-test", "2-supply", "03-package"},
			valido:    false,
			enElError: []string{"steps/2-supply"},
			nota:      "HOY se ejecuta en el orden 1,3,2",
		},
		{
			nombre:    "prefijo de tres dígitos",
			entradas:  []string{"002-supply"},
			valido:    false,
			enElError: []string{"steps/002-supply"},
			nota:      "HOY colapsa en la misma clave de estado que 02-supply",
		},
		{
			nombre:    "separador guion bajo",
			entradas:  []string{"01-test", "4_test"},
			valido:    false,
			enElError: []string{"steps/4_test"},
			nota:      "HOY desaparece en silencio",
		},
		{
			nombre:    "sin prefijo numérico",
			entradas:  []string{"supply"},
			valido:    false,
			enElError: []string{"steps/supply"},
		},
		{
			nombre:    "el número detrás del nombre",
			entradas:  []string{"package-02"},
			valido:    false,
			enElError: []string{"steps/package-02"},
		},
		{
			nombre:    "prefijo vacío",
			entradas:  []string{"-deploy"},
			valido:    false,
			enElError: []string{"steps/-deploy"},
		},
		{
			nombre:    "nombre vacío",
			entradas:  []string{"02-"},
			valido:    false,
			enElError: []string{"steps/02-"},
		},
		{
			nombre:    "dos steps con el mismo orden",
			entradas:  []string{"01-test", "02-a", "02-b"},
			valido:    false,
			enElError: []string{"steps/02-a", "steps/02-b", "orden 02"},
			nota:      "HOY el orden de ejecución entre los dos es arbitrario y comparten clave de estado",
		},
		{
			nombre:    "varios problemas se reportan de una vez",
			entradas:  []string{"2-supply", "4_test", "03-a", "03-b"},
			valido:    false,
			enElError: []string{"steps/2-supply", "steps/4_test", "steps/03-a", "steps/03-b"},
			nota:      "la validación no corta en el primer fallo (§5.3')",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			err := validador.Validate(ctxDePrueba(), rutaDePrueba, entradas(caso.entradas...))

			if caso.valido {
				require.NoError(t, err, caso.nota)
				return
			}
			require.Error(t, err, caso.nota)
			for _, fragmento := range caso.enElError {
				assert.Contains(t, err.Error(), fragmento,
					"el error tiene que nombrar al culpable. %s", caso.nota)
			}
		})
	}
}

func TestPipelineStructureValidator_ReglasComponibles(t *testing.T) {
	// OCP (§5.2'): las specs 05 y 15 añaden reglas sin tocar el repositorio ni el
	// handler. Este test fija que el validador es una composición y no una cascada
	// de `if`.
	t.Run("un validador sin reglas acepta cualquier cosa", func(t *testing.T) {
		require.NoError(t,
			domPipeline.NewStepsStructureValidator().Validate(
				ctxDePrueba(), rutaDePrueba, entradas("4_test", "02-a", "02-b")))
	})

	t.Run("cada regla se puede verificar por separado", func(t *testing.T) {
		soloFormato := domPipeline.NewStepsStructureValidator(domPipeline.NewStepEntryFormatRule())
		soloOrden := domPipeline.NewStepsStructureValidator(domPipeline.NewUniqueStepOrderRule())

		assert.Error(t, soloFormato.Validate(ctxDePrueba(), rutaDePrueba, entradas("4_test")))
		assert.NoError(t, soloOrden.Validate(ctxDePrueba(), rutaDePrueba, entradas("4_test")),
			"un directorio ilegible no es un orden duplicado: de él habla la otra regla")

		assert.NoError(t, soloFormato.Validate(ctxDePrueba(), rutaDePrueba, entradas("02-a", "02-b")))
		assert.Error(t, soloOrden.Validate(ctxDePrueba(), rutaDePrueba, entradas("02-a", "02-b")))
	})

	t.Run("una regla añadida se evalúa", func(t *testing.T) {
		validador := domPipeline.NewStepsStructureValidator(reglaQueSiempreFalla{})

		err := validador.Validate(ctxDePrueba(), rutaDePrueba, entradas("01-test"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "regla_de_prueba", "el error identifica la regla que falló")
	})
}

// ── La regla del ámbito declarado (spec 13) ─────────────────────────────────

// Lo que la regla exige NO es que el `config.yaml` exista: es que el que exista
// diga algo del vocabulario cerrado. La diferencia es la mitad de §5.3 que es
// fácil de perder, porque las dos situaciones se parecen desde fuera.
func TestStepScopeRule_QueSeExigeDeLoDeclarado(t *testing.T) {
	casos := []struct {
		nombre    string
		declarado map[string]string // directorio → contenido de `scope:`
		valido    bool
		enElError []string
		nota      string
	}{
		{
			nombre:    "ningún step declara nada",
			declarado: nil,
			valido:    true,
			nota:      "sin config.yaml el step se ejecuta siempre y no persiste: legítimo (§5.3)",
		},
		{
			nombre:    "los dos ámbitos del vocabulario",
			declarado: map[string]string{"01-acr": "project", "02-provision": "environment"},
			valido:    true,
		},
		{
			nombre:    "un ámbito inventado",
			declarado: map[string]string{"02-supply": "shared"},
			valido:    false,
			enElError: []string{"steps/02-supply/config.yaml", "shared"},
			nota:      "un ámbito inventado no tendría dónde persistirse",
		},
		{
			nombre:    "config.yaml presente sin scope",
			declarado: map[string]string{"02-supply": ""},
			valido:    false,
			enElError: []string{"steps/02-supply/config.yaml"},
			nota:      "presente es una intención de declarar: tratarlo como ausente sería adivinar cuál",
		},
		{
			nombre:    "sólo uno de varios está mal",
			declarado: map[string]string{"01-test": "environment", "02-supply": "Project"},
			valido:    false,
			enElError: []string{"steps/02-supply/config.yaml", "Project"},
			nota:      "el vocabulario es sensible a mayúsculas",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			nombres := make([]string, 0, len(caso.declarado))
			for dir := range caso.declarado {
				nombres = append(nombres, dir)
			}
			if len(nombres) == 0 {
				nombres = []string{"01-test", "02-supply"}
			}

			validador := domPipeline.NewStepsStructureValidator(
				domPipeline.NewStepScopeRule(configsDeclarados(caso.declarado)))

			err := validador.Validate(ctxDePrueba(), rutaDePrueba, entradas(nombres...))

			if caso.valido {
				require.NoError(t, err, caso.nota)
				return
			}
			require.Error(t, err, caso.nota)
			for _, fragmento := range caso.enElError {
				assert.Contains(t, err.Error(), fragmento,
					"el error tiene que nombrar al culpable. %s", caso.nota)
			}
		})
	}
}

// configsDeclarados imita al repositorio real: valida el vocabulario al
// traducir, y nombra el archivo en su error.
type configsDeclarados map[string]string

var _ domStep.StepConfigRepository = configsDeclarados(nil)

func (c configsDeclarados) Get(_ *context.Context, _, step string) (domStep.StepConfig, error) {
	declarado, presente := c[step]
	if !presente {
		return domStep.NoStepConfig(), nil
	}
	scope, err := domStep.NewScope(declarado)
	if err != nil {
		return domStep.StepConfig{}, fmt.Errorf("'steps/%s/config.yaml' %w", step, err)
	}
	return domStep.NewStepConfig(scope)
}

type reglaQueSiempreFalla struct{}

var _ domPipeline.StepStructureRule = (*reglaQueSiempreFalla)(nil)

func (reglaQueSiempreFalla) Name() string { return "regla_de_prueba" }

func (reglaQueSiempreFalla) IsSatisfiedBy(*context.Context, string, []domPipeline.StepEntry) error {
	return assert.AnError
}
