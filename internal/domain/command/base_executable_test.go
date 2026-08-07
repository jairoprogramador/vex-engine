package command_test

// La limpieza del Template Method ocurre pase lo que pase con `exec` (spec 06).
//
// El defecto no era teórico: `after` es el único punto de limpieza de las tres
// cadenas —restaurar las plantillas interpoladas, retirar `step_workdir`— y
// estaba detrás del camino feliz. O sea que no corría justo en el caso que lo
// necesita, que es el fallo, y que además es el caso normal.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

var (
	errExec   = errors.New("el comando salió con 3")
	errAfter  = errors.New("no se pudo restaurar la plantilla")
	errBefore = errors.New("no se pudo abrir la sesión")
)

// El test que fallaba antes de la spec 06: la plantilla interpolada tiene que
// volver a su contenido original aunque el comando falle.
//
// Se monta con una FileInterpolatorSession REAL sobre un archivo temporal, no
// con un doble, porque lo que se afirma es lo que queda en disco: una plantilla
// que se queda con los valores sustituidos ya no tiene los `${var.…}` que la
// ejecución siguiente busca, y el segundo fallo es más confuso que el primero.
func TestBaseExecutable_UnExecFallidoDejaLaPlantillaEnSuContenidoOriginal(t *testing.T) {
	original := "image: ${var.registry_prefix}/demo-app\n"
	ruta := filepath.Join(t.TempDir(), "deployment.yaml")
	require.NoError(t, os.WriteFile(ruta, []byte(original), 0o644))

	vars := command.NewExecutionVariableMap()
	prefijo, err := command.NewVariable("registry_prefix", "vexsand", false, command.OriginDeclared)
	require.NoError(t, err)
	vars.Add(prefijo)

	interpolador := command.NewFileInterpolator(discoDePrueba{})
	var ejecutable command.BaseExecutable
	var sesion command.FileInterpolatorSession

	err = ejecutable.Run(nil,
		func() error {
			var err error
			sesion, err = interpolador.Interpolate([]string{ruta}, vars)
			return err
		},
		func() error {
			// La plantilla está interpolada mientras el comando corre: es
			// exactamente para eso que se interpola.
			contenido, err := os.ReadFile(ruta)
			require.NoError(t, err)
			require.Equal(t, "image: vexsand/demo-app\n", string(contenido))
			return errExec
		},
		func() error { return sesion.Restore() },
	)

	require.ErrorIs(t, err, errExec)

	contenido, errLectura := os.ReadFile(ruta)
	require.NoError(t, errLectura)
	assert.Equal(t, original, string(contenido),
		"el workdir se reutiliza en la ejecución siguiente: una plantilla ya interpolada ya no tiene ${var.…} que interpolar")
}

func TestBaseExecutable_ComposicionDeErrores(t *testing.T) {
	// Regla de precedencia (spec 06 §8): el error de `exec` gana, porque es la
	// causa; el de `after` es la consecuencia y se acompaña, no se pierde.
	t.Run("fallan exec y after: manda el de exec y el de after sigue ahí", func(t *testing.T) {
		var ejecutable command.BaseExecutable

		err := ejecutable.Run(nil, nil,
			func() error { return errExec },
			func() error { return errAfter },
		)

		require.ErrorIs(t, err, errExec)
		require.ErrorIs(t, err, errAfter)
		assert.True(t, strings.HasPrefix(err.Error(), errExec.Error()),
			"el mensaje empieza por la causa, no por la consecuencia: %q", err.Error())
	})

	// El único caso en que `after` es la causa.
	t.Run("exec tiene éxito y after falla: manda el de after", func(t *testing.T) {
		var ejecutable command.BaseExecutable

		err := ejecutable.Run(nil, nil,
			func() error { return nil },
			func() error { return errAfter },
		)

		require.ErrorIs(t, err, errAfter)
		assert.Equal(t, errAfter.Error(), err.Error())
	})

	t.Run("nadie falla: sin error y after corrió igual", func(t *testing.T) {
		var ejecutable command.BaseExecutable
		vecesAfter := 0

		require.NoError(t, ejecutable.Run(nil,
			func() error { return nil },
			func() error { return nil },
			func() error { vecesAfter++; return nil },
		))
		assert.Equal(t, 1, vecesAfter)
	})
}

// `after` no corre si `before` falló, y es deliberado: `before` es quien crea lo
// que `after` limpia. Restaurar sesiones que no se abrieron no es limpieza, es
// una operación sobre estado que no existe.
func TestBaseExecutable_BeforeFallidoNoDisparaLaLimpieza(t *testing.T) {
	var ejecutable command.BaseExecutable
	vecesExec, vecesAfter := 0, 0

	err := ejecutable.Run(nil,
		func() error { return errBefore },
		func() error { vecesExec++; return nil },
		func() error { vecesAfter++; return nil },
	)

	require.ErrorIs(t, err, errBefore)
	assert.Zero(t, vecesExec)
	assert.Zero(t, vecesAfter, "no hay sesión abierta que restaurar")
}

// ── Dobles ──────────────────────────────────────────────────────────────────

// discoDePrueba es el FileSystem del dominio sobre el disco real. El test
// necesita observar el archivo, así que no sirve un doble en memoria.
type discoDePrueba struct{}

var _ command.FileSystem = discoDePrueba{}

func (discoDePrueba) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (discoDePrueba) WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
