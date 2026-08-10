package record_test

// Lo que la spec 19 añade al modelo: la clasificación del fallo, el resumen del
// valor y el vocabulario de motivos.
//
// Los tres viven en dominio puro y sin I/O por la misma razón que `Fold`: son la
// traducción de lo OBSERVADO al vocabulario cerrado, y una traducción distinta
// por emisor produciría un agregado que cuenta el mismo fallo de tres maneras.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/record"
)

func TestClassifyError(t *testing.T) {
	t.Run("sin error no hay clase, y ése es el valor cero", func(t *testing.T) {
		clase := record.ClassifyError(nil)
		assert.Equal(t, record.ErrorClassNone, clase)
		assert.True(t, clase.IsZero())
	})

	t.Run("el comando que devolvió un código distinto de cero", func(t *testing.T) {
		err := command.NewCommandFailedError("mvn test", 3, "BUILD FAILURE")

		assert.Equal(t, record.ErrorClassCommandFailed, record.ClassifyError(err))

		codigo, ok := record.ExitCodeOf(err)
		require.True(t, ok)
		assert.Equal(t, 3, codigo)
	})

	t.Run("y sigue clasificándose envuelto, que es como viaja", func(t *testing.T) {
		err := fmt.Errorf("ejecutar el step: %w",
			command.NewCommandFailedError("mvn test", 3, ""))

		assert.Equal(t, record.ErrorClassCommandFailed, record.ClassifyError(err))
		codigo, _ := record.ExitCodeOf(err)
		assert.Equal(t, 3, codigo)
	})

	// LA CANCELACIÓN GANA AL FALLO DEL COMANDO, y no es un detalle de orden: al
	// cancelar el contexto el comando en curso MUERE y devuelve un código, así que
	// mirar primero el comando contaría una decisión como una desgracia.
	t.Run("una cancelación no es el fallo del comando que mató", func(t *testing.T) {
		err := errors.Join(
			context.Canceled,
			command.NewCommandFailedError("terraform apply", 130, ""))

		assert.Equal(t, record.ErrorClassCancelled, record.ClassifyError(err))
	})

	// `unknown` es un valor legítimo y preferible a forzar: un fallo que el motor
	// no sabe clasificar es exactamente eso, y decirlo es un hecho.
	t.Run("lo que no se sabe clasificar se dice", func(t *testing.T) {
		err := errors.New("clonar el proyecto: repositorio no encontrado")

		assert.Equal(t, record.ErrorClassUnknown, record.ClassifyError(err))

		_, ok := record.ExitCodeOf(err)
		assert.False(t, ok, "un fallo que no fue de un comando no tiene código propio")
	})
}

// ── El resumen de un parámetro (spec 20 §5.2) ───────────────────────────────

const (
	proyectoUno = "11111111-1111-1111-1111-111111111111"
	proyectoDos = "22222222-2222-2222-2222-222222222222"
)

// secretoDePrueba es un secreto fijo: lo que estos casos miden es la regla de
// derivación, no de dónde sale el secreto —eso es de infraestructura, y lo mide
// `TestReadOrCreateDigestSecret_*`—.
func secretoDePrueba(t *testing.T, marca byte) []byte {
	t.Helper()
	secreto := make([]byte, 32)
	for i := range secreto {
		secreto[i] = marca
	}
	return secreto
}

func resumidor(t *testing.T, marca byte, proyecto string) *record.ParameterDigester {
	t.Helper()
	d, err := record.NewParameterDigester(secretoDePrueba(t, marca))
	require.NoError(t, err)
	require.NoError(t, d.Bind(proyecto))
	return d
}

func resumen(t *testing.T, d *record.ParameterDigester, valor string) string {
	t.Helper()
	digest, err := d.Digest(valor)
	require.NoError(t, err)
	return digest
}

func TestParameterDigester(t *testing.T) {
	t.Run("el valor no entra: entra su resumen", func(t *testing.T) {
		digest := resumen(t, resumidor(t, 0x01, proyectoUno), "Server=tcp:prod;Password=s3cr3t")

		assert.True(t, strings.HasPrefix(digest, record.DigestVersion+":"),
			"el prefijo es lo que impide que un resumen viejo y uno nuevo se comparen como iguales")
		assert.NotContains(t, digest, "s3cr3t",
			"un registro que viaja fuera de la organización no puede llevar secretos en claro")
	})

	// Es la propiedad que hace útil el hecho: «¿corrió con lo mismo que ayer?».
	t.Run("es estable y sensible dentro del proyecto", func(t *testing.T) {
		uno := resumidor(t, 0x01, proyectoUno)
		otro := resumidor(t, 0x01, proyectoUno)

		assert.Equal(t, resumen(t, uno, "demo-app"), resumen(t, otro, "demo-app"),
			"dos ejecuciones del mismo proyecto tienen que poder compararse")
		assert.NotEqual(t, resumen(t, uno, "demo-app"), resumen(t, uno, "demo-app2"))
	})

	// **La razón de ser de la spec 20 §5.2.** Con SHA-256 desnudo estos dos
	// resúmenes serían idénticos y triviales de invertir: `sha256("3")` es una
	// búsqueda en una tabla, no un secreto.
	t.Run("dos proyectos con el mismo valor dan resúmenes distintos", func(t *testing.T) {
		uno := resumidor(t, 0x01, proyectoUno)
		dos := resumidor(t, 0x01, proyectoDos)

		assert.NotEqual(t, resumen(t, uno, "3"), resumen(t, dos, "3"),
			"la clave es POR PROYECTO: sin eso un valor de baja entropía es reversible")
	})

	// Y dos instalaciones tampoco coinciden, que es lo que impide construir una
	// tabla de resúmenes válida en todas partes.
	t.Run("dos secretos distintos dan resúmenes distintos", func(t *testing.T) {
		assert.NotEqual(t,
			resumen(t, resumidor(t, 0x01, proyectoUno), "3"),
			resumen(t, resumidor(t, 0x02, proyectoUno), "3"))
	})

	// «Se resolvió a la cadena vacía» es un hecho distinto de «no se resolvió», y
	// colapsarlos en la ausencia de digest perdería justo el caso que suele ser el
	// defecto (spec 03 §5.3).
	t.Run("la cadena vacía también tiene resumen", func(t *testing.T) {
		d := resumidor(t, 0x01, proyectoUno)
		assert.NotEmpty(t, resumen(t, d, ""))
		assert.NotEqual(t, resumen(t, d, ""), resumen(t, d, " "))
	})

	// Un resumen sin clave sería un resumen SIN SAL, que es exactamente el defecto
	// que esta spec quita de en medio. Se falla en vez de degradarse.
	t.Run("sin proyecto no hay resumen, y eso es un error", func(t *testing.T) {
		d, err := record.NewParameterDigester(secretoDePrueba(t, 0x01))
		require.NoError(t, err)

		_, err = d.Digest("3")
		require.Error(t, err)

		require.Error(t, d.Bind(""), "un proyecto vacío no deriva ninguna clave")
	})

	t.Run("enlazar dos veces es un error del cableado", func(t *testing.T) {
		d := resumidor(t, 0x01, proyectoUno)
		require.Error(t, d.Bind(proyectoDos),
			"una ejecución es de UN proyecto: cambiarlo a mitad dejaría dos resúmenes del mismo valor")
	})

	t.Run("un secreto corto no se acepta", func(t *testing.T) {
		_, err := record.NewParameterDigester([]byte("corto"))
		require.Error(t, err)
	})
}

func TestStepReason_ElVocabularioEsCerrado(t *testing.T) {
	conocidos := []command.StepReason{
		command.ReasonNone,
		command.ReasonNoCommands,
		command.ReasonNoScope,
		command.ReasonNoRules,
		command.ReasonNoRecord,
		command.ReasonChanged,
		command.ReasonExpired,
		command.ReasonUndetermined,
		command.ReasonUpToDate,
	}
	for _, motivo := range conocidos {
		assert.True(t, motivo.IsKnown(), "%q debería estar en el vocabulario", motivo)
	}

	assert.False(t, command.StepReason("porque sí").IsKnown())

	// Y `undetermined` NO es `changed`: «no se pudo componer la huella» ejecuta
	// como «el contenido cambió» y no se disfraza de ello (spec 15). Un booleano
	// perdería exactamente esa distinción.
	assert.NotEqual(t, command.ReasonChanged, command.ReasonUndetermined)
}

// Un motivo fuera del vocabulario es un error DEL EMISOR y no un hecho
// degradado: el consumidor agrega por este campo, y un valor inventado se cuenta
// aparte para siempre sin que nadie sepa de dónde salió.
func TestStepFinished_RechazaUnMotivoQueNoEsDelVocabulario(t *testing.T) {
	carga := record.StepFinished{
		StepID: "02-supply",
		Status: command.StepSuccess,
		Reason: command.StepReason("me lo he inventado"),
	}

	err := carga.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vocabulario")
}

// La ausencia de motivo es LEGÍTIMA, y hay un camino real que la produce: un step
// cuyo `before` falla nunca llegó a que nadie decidiera nada sobre él.
func TestStepFinished_LaAusenciaDeMotivoEsLegitima(t *testing.T) {
	carga := record.StepFinished{
		StepID:     "02-supply",
		Status:     command.StepFailure,
		ErrorClass: record.ErrorClassUnknown,
	}

	require.NoError(t, carga.Validate())
}
