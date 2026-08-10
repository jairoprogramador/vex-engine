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

func TestDigestOf(t *testing.T) {
	t.Run("el valor no entra: entra su resumen", func(t *testing.T) {
		digest := record.DigestOf("Server=tcp:prod;Password=s3cr3t")

		assert.True(t, strings.HasPrefix(digest, "sha256:"),
			"el prefijo es lo que deja a la spec 20 cambiar de convención sin que los viejos mientan")
		assert.NotContains(t, digest, "s3cr3t",
			"un registro que viaja fuera de la organización no puede llevar secretos en claro")
	})

	t.Run("es determinista y sensible", func(t *testing.T) {
		assert.Equal(t, record.DigestOf("demo-app"), record.DigestOf("demo-app"),
			"dos corridas con el mismo valor tienen que poder compararse")
		assert.NotEqual(t, record.DigestOf("demo-app"), record.DigestOf("demo-app2"))
	})

	// «Se resolvió a la cadena vacía» es un hecho distinto de «no se resolvió», y
	// colapsarlos en la ausencia de digest perdería justo el caso que suele ser el
	// defecto.
	t.Run("la cadena vacía también tiene resumen", func(t *testing.T) {
		assert.NotEmpty(t, record.DigestOf(""))
		assert.NotEqual(t, record.DigestOf(""), record.DigestOf(" "))
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
