package pipeline_test

// El invariante estructural del pipelinecode (spec 04 §5.1).
//
// Cada caso de esta tabla es un pipelinecode que HOY —antes de la spec 04— se
// ejecutaba con exit code 0 y un step de menos, o con los steps reordenados. La
// diferencia observable no es que falle: es que el mensaje nombra el directorio
// culpable.

import (
	"testing"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipelineStructureValidator_QueEstructuraSeAcepta(t *testing.T) {
	validador := domPipeline.NewPipelineStructureValidator()

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
			err := validador.Validate(entradas(caso.entradas...))

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
			domPipeline.NewStepsStructureValidator().Validate(entradas("4_test", "02-a", "02-b")))
	})

	t.Run("cada regla se puede verificar por separado", func(t *testing.T) {
		soloFormato := domPipeline.NewStepsStructureValidator(domPipeline.NewStepEntryFormatRule())
		soloOrden := domPipeline.NewStepsStructureValidator(domPipeline.NewUniqueStepOrderRule())

		assert.Error(t, soloFormato.Validate(entradas("4_test")))
		assert.NoError(t, soloOrden.Validate(entradas("4_test")),
			"un directorio ilegible no es un orden duplicado: de él habla la otra regla")

		assert.NoError(t, soloFormato.Validate(entradas("02-a", "02-b")))
		assert.Error(t, soloOrden.Validate(entradas("02-a", "02-b")))
	})

	t.Run("una regla añadida se evalúa", func(t *testing.T) {
		validador := domPipeline.NewStepsStructureValidator(reglaQueSiempreFalla{})

		err := validador.Validate(entradas("01-test"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "regla_de_prueba", "el error identifica la regla que falló")
	})
}

type reglaQueSiempreFalla struct{}

var _ domPipeline.StepStructureRule = (*reglaQueSiempreFalla)(nil)

func (reglaQueSiempreFalla) Name() string { return "regla_de_prueba" }

func (reglaQueSiempreFalla) IsSatisfiedBy([]domPipeline.StepEntry) error {
	return assert.AnError
}
