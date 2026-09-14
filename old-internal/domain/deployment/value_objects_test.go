package deployment_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
)

func TestSubject(t *testing.T) {
	t.Run("una url de proyecto es un sujeto", func(t *testing.T) {
		subject, err := deployment.NewSubject("https://vex.test/acme/demo-app")
		require.NoError(t, err)

		assert.Equal(t, "https://vex.test/acme/demo-app", subject.String())
		assert.False(t, subject.IsZero())
	})

	t.Run("un sujeto vacío es un error y no un sujeto degradado", func(t *testing.T) {
		_, err := deployment.NewSubject("")
		require.Error(t, err)
	})

	t.Run("el valor cero no es una identidad", func(t *testing.T) {
		var subject deployment.Subject
		assert.True(t, subject.IsZero())
		assert.Empty(t, subject.String())
	})

	t.Run("la igualdad es por valor", func(t *testing.T) {
		uno, err := deployment.NewSubject("https://vex.test/acme/demo-app")
		require.NoError(t, err)
		otro, err := deployment.NewSubject("https://vex.test/acme/otra")
		require.NoError(t, err)

		assert.True(t, uno.Equals(uno))
		assert.False(t, uno.Equals(otro))
	})
}

func TestDestination(t *testing.T) {
	t.Run("un ambiente es un destino", func(t *testing.T) {
		destino, err := deployment.NewDestination("prod")
		require.NoError(t, err)

		assert.Equal(t, "prod", destino.String())
		assert.False(t, destino.IsZero())
		assert.True(t, destino.Equals(destino))
	})

	// NINGUNA palabra del usuario está reservada, y `shared` la menos.
	//
	// La spec 04 §5.4 la prohibió para arreglar una colisión de rutas y la
	// spec 13 §5.5 DEROGÓ la reserva al cambiar la clave: el ámbito de ambiente
	// viaja siempre prefijado, así que `shared` produce `environment:shared` y no
	// pisa a nadie. Este caso afirmaba lo contrario —la 17 reintrodujo la reserva
	// sin que ningún camino de ejecución construyera todavía un `Destination`— y
	// se invierte con la spec 18, que es la primera que lo construye de verdad.
	t.Run("shared es un ambiente como cualquier otro", func(t *testing.T) {
		destino, err := deployment.NewDestination("shared")
		require.NoError(t, err)
		assert.Equal(t, "shared", destino.String())
	})

	t.Run("un nombre que rompería una ruta es un error, no una ruta rara", func(t *testing.T) {
		for _, nombre := range []string{"", "pro/duccion", `pro\duccion`, "pro:duccion", ".", ".."} {
			_, err := deployment.NewDestination(nombre)
			require.Error(t, err, "se esperaba error para %q", nombre)
		}
	})

	t.Run("el valor cero no es un destino", func(t *testing.T) {
		var destino deployment.Destination
		assert.True(t, destino.IsZero())
		assert.Empty(t, destino.String())
	})
}

func TestOperation(t *testing.T) {
	t.Run("cualquier nombre de step es una operación", func(t *testing.T) {
		for _, nombre := range []string{"deploy", "supply", "notify", "05-notify"} {
			operacion, err := deployment.NewOperation(nombre)
			require.NoError(t, err, "se esperaba operación para %q", nombre)
			assert.Equal(t, nombre, operacion.String())
		}
	})

	t.Run("vacía o con espacios es un error", func(t *testing.T) {
		for _, nombre := range []string{"", " deploy", "deploy ", "de ploy", "de\tploy"} {
			_, err := deployment.NewOperation(nombre)
			require.Error(t, err, "se esperaba error para %q", nombre)
		}
	})

	t.Run("el valor cero no es una operación", func(t *testing.T) {
		var operacion deployment.Operation
		assert.True(t, operacion.IsZero())
		assert.Empty(t, operacion.String())

		otra, err := deployment.NewOperation("deploy")
		require.NoError(t, err)
		assert.False(t, operacion.Equals(otra))
	})
}

func TestSource(t *testing.T) {
	proyecto := huella(t, fingerprint.Version, "33")
	pipeline := huella(t, fingerprint.Version, "44")

	t.Run("las dos huellas componen la fuente", func(t *testing.T) {
		fuente, err := deployment.NewSource(proyecto, pipeline)
		require.NoError(t, err)

		assert.True(t, fuente.Project().Equals(proyecto))
		assert.True(t, fuente.Pipeline().Equals(pipeline))
		assert.False(t, fuente.IsZero())
		assert.True(t, fuente.Equals(fuente))
	})

	t.Run("una fuente a medias es un error, nunca una identidad degradada", func(t *testing.T) {
		_, err := deployment.NewSource(fingerprint.Fingerprint{}, pipeline)
		require.Error(t, err)

		_, err = deployment.NewSource(proyecto, fingerprint.Fingerprint{})
		require.Error(t, err)
	})

	t.Run("dos fuentes con las huellas cruzadas no son iguales", func(t *testing.T) {
		una, err := deployment.NewSource(proyecto, pipeline)
		require.NoError(t, err)
		otra, err := deployment.NewSource(pipeline, proyecto)
		require.NoError(t, err)

		assert.False(t, una.Equals(otra))
	})

	t.Run("el valor cero no es una fuente", func(t *testing.T) {
		var fuente deployment.Source
		assert.True(t, fuente.IsZero())
	})
}

func TestFormat(t *testing.T) {
	t.Run("la versión declarada y la ausencia del manifiesto son dos formatos", func(t *testing.T) {
		declarado, err := deployment.NewFormat(2, true)
		require.NoError(t, err)
		sinDeclarar, err := deployment.NewFormat(1, false)
		require.NoError(t, err)

		assert.Equal(t, 2, declarado.SchemaVersion())
		assert.True(t, declarado.IsDeclared())
		assert.False(t, sinDeclarar.IsDeclared())
		assert.False(t, declarado.Equals(sinDeclarar))
		assert.False(t, declarado.IsZero())
	})

	t.Run("una versión que no lo es, es un error", func(t *testing.T) {
		_, err := deployment.NewFormat(0, true)
		require.Error(t, err)

		_, err = deployment.NewFormat(-1, false)
		require.Error(t, err)
	})

	t.Run("una versión que este motor no conoce se REPRESENTA, no se rechaza", func(t *testing.T) {
		// Rechazarla es de `pipeline.NewManifest`, que es quien lee el archivo.
		// Un registro que no sabe describir lo que pasó no sirve de registro.
		futuro, err := deployment.NewFormat(99, true)
		require.NoError(t, err)
		assert.Equal(t, 99, futuro.SchemaVersion())
	})

	t.Run("el valor cero no es un formato", func(t *testing.T) {
		var formato deployment.Format
		assert.True(t, formato.IsZero())
	})
}

func TestAttempt(t *testing.T) {
	t.Run("el primer intento es el 1", func(t *testing.T) {
		primero := deployment.FirstAttempt()
		assert.Equal(t, 1, primero.Number())
		assert.Equal(t, "1", primero.String())
		assert.False(t, primero.IsZero())
	})

	t.Run("el siguiente intento incrementa", func(t *testing.T) {
		assert.Equal(t, 2, deployment.FirstAttempt().Next().Number())
	})

	t.Run("un reintento se construye por su número", func(t *testing.T) {
		cuarto, err := deployment.NewAttempt(4)
		require.NoError(t, err)

		assert.Equal(t, 4, cuarto.Number())
		assert.Equal(t, "4", cuarto.String())
		assert.True(t, cuarto.Equals(deployment.FirstAttempt().Next().Next().Next()))
	})

	t.Run("cero y negativo no son intentos", func(t *testing.T) {
		_, err := deployment.NewAttempt(0)
		require.Error(t, err)

		_, err = deployment.NewAttempt(-3)
		require.Error(t, err)
	})

	t.Run("el valor cero significa que no consta", func(t *testing.T) {
		var intento deployment.Attempt
		assert.True(t, intento.IsZero())
		assert.Empty(t, intento.String())
		assert.False(t, intento.Equals(deployment.FirstAttempt()))
	})
}

// huella construye una huella con el hash fijo `par` repetido 32 veces, bajo la
// versión pedida. Los literales fijos son deliberados: los vectores de este
// paquete se tienen que poder validar sin reimplementar antes las tres reglas de
// huella.
func huella(t *testing.T, version, par string) fingerprint.Fingerprint {
	t.Helper()

	value, err := fingerprint.Parse(version + ":" + strings.Repeat(par, 32))
	require.NoError(t, err)
	return value
}
