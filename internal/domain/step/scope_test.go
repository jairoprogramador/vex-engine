package step_test

// El ámbito que un step DECLARA (spec 13 §5.1).
//
// Dos papeles que no hay que confundir, y los dos se prueban aquí: el VALOR
// declarado —vocabulario cerrado, lo que se escribe en el `config.yaml`— y la
// DIRECCIÓN que de él se deriva —la clave de estado, `state.Scope`—. Que el
// value object exista en un solo sitio es lo que permite que las dos lecturas
// salgan del mismo dato en vez de divergir, que es lo que pasó con `shared`.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domState "github.com/jairoprogramador/vex-engine/internal/domain/state"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

func TestNewScope_ElVocabularioEsCerrado(t *testing.T) {
	casos := []struct {
		declarado string
		valido    bool
		nota      string
	}{
		{declarado: "project", valido: true},
		{declarado: "environment", valido: true},
		{declarado: "", valido: false, nota: "un config.yaml presente que no declara nada"},
		{declarado: "shared", valido: false,
			nota: "la palabra que el mecanismo viejo usaba: ya no significa nada para el motor"},
		{declarado: "Project", valido: false, nota: "sensible a mayúsculas"},
		{declarado: "environment:prod", valido: false,
			nota: "la forma LÓGICA de state.Scope no es lo que un step declara: el ambiente lo pone la ejecución"},
		{declarado: "global", valido: false,
			nota: "un ámbito inventado no tendría dónde persistirse"},
	}

	for _, caso := range casos {
		t.Run(caso.declarado, func(t *testing.T) {
			scope, err := domStep.NewScope(caso.declarado)

			if !caso.valido {
				require.Error(t, err, caso.nota)
				assert.True(t, scope.IsZero())
				return
			}
			require.NoError(t, err, caso.nota)
			assert.Equal(t, caso.declarado, scope.String(),
				"la forma declarada es la que se escribe en el config.yaml")
			assert.False(t, scope.IsZero())
		})
	}
}

// El error nombra el vocabulario esperado. Quien lo lee está editando un
// `config.yaml`, y «scope inválido» sin decir cuáles valen le obliga a buscar la
// spec.
func TestNewScope_ElErrorDiceQueSeEsperaba(t *testing.T) {
	_, err := domStep.NewScope("shared")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "project")
	assert.Contains(t, err.Error(), "environment")
}

// La traducción del valor declarado a la DIRECCIÓN. El ambiente sólo hace falta
// para uno de los dos ámbitos, y que el de proyecto lo ignore es exactamente lo
// que hace que dos despliegues a ambientes distintos escriban en el mismo sitio.
func TestScope_StateScope(t *testing.T) {
	t.Run("project ignora el ambiente", func(t *testing.T) {
		unAmbiente, err := domStep.NewProjectScope().StateScope("sand")
		require.NoError(t, err)
		otroAmbiente, err := domStep.NewProjectScope().StateScope("prod")
		require.NoError(t, err)

		assert.True(t, unAmbiente.Equals(otroAmbiente),
			"el ACR pertenece al proyecto: los dos despliegues escriben en el mismo sitio")
		assert.True(t, unAmbiente.Equals(domState.NewProjectScope()))
		assert.Equal(t, []string{"project"}, unAmbiente.Segments())
	})

	t.Run("environment lleva el ambiente en ejecución", func(t *testing.T) {
		scope, err := domStep.NewEnvironmentScope().StateScope("sand")
		require.NoError(t, err)

		assert.Equal(t, "environment:sand", scope.String())
		assert.Equal(t, []string{"environment", "sand"}, scope.Segments(),
			"DOS segmentos en la ruta: `:` es ilegal en rutas de Windows")
	})

	// La reserva de `shared` se deroga (§5.5), pero la validación del NOMBRE del
	// ambiente NO: es la que impide que un `value: pro/duccion` produzca un
	// directorio anidado dentro del almacén.
	t.Run("el nombre del ambiente se sigue validando", func(t *testing.T) {
		for _, ambiente := range []string{"", "pro/duccion", `pro\duccion`, "pro:duccion", ".", ".."} {
			t.Run(ambiente, func(t *testing.T) {
				_, err := domStep.NewEnvironmentScope().StateScope(ambiente)
				assert.Error(t, err)
			})
		}

		for _, ambiente := range []string{"shared", "project", "environment"} {
			t.Run(ambiente+" (ya no reservado)", func(t *testing.T) {
				scope, err := domStep.NewEnvironmentScope().StateScope(ambiente)
				require.NoError(t, err)
				assert.False(t, scope.Equals(domState.NewProjectScope()),
					"viaja prefijado, así que no colisiona con el ámbito de proyecto")
			})
		}
	})

	t.Run("el ámbito cero no tiene dirección", func(t *testing.T) {
		_, err := domStep.Scope{}.StateScope("sand")
		assert.Error(t, err, "un step sin config.yaml no tiene dónde escribir, y eso no se resuelve inventando")
	})
}

// `NoStepConfig` es el step sin `config.yaml`, y su valor cero tiene que ser el
// seguro: no declara, luego no persiste.
func TestStepConfig_SinArchivoNoDeclaraNada(t *testing.T) {
	assert.False(t, domStep.NoStepConfig().IsDeclared())
	assert.True(t, domStep.NoStepConfig().Scope().IsZero())

	config, err := domStep.NewStepConfig(domStep.NewProjectScope())
	require.NoError(t, err)
	assert.True(t, config.IsDeclared())
	assert.True(t, config.Scope().IsProject())

	_, err = domStep.NewStepConfig(domStep.Scope{})
	assert.Error(t, err, "una configuración declarada sin ámbito no es construible")
}
