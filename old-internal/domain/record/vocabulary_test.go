package record_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
)

func TestSeq(t *testing.T) {
	t.Run("la primera posición es la 1", func(t *testing.T) {
		primera := record.FirstSeq()
		assert.Equal(t, uint64(1), primera.Position())
		assert.Equal(t, "1", primera.String())
		assert.False(t, primera.IsZero())
	})

	t.Run("la siguiente incrementa y ordena", func(t *testing.T) {
		primera := record.FirstSeq()
		segunda := primera.Next()

		assert.True(t, primera.Before(segunda))
		assert.False(t, segunda.Before(primera))
		assert.False(t, primera.Equals(segunda))
	})

	t.Run("se construye por su número", func(t *testing.T) {
		septima, err := record.NewSeq(7)
		require.NoError(t, err)
		assert.Equal(t, uint64(7), septima.Position())
	})

	t.Run("el cero no es una posición", func(t *testing.T) {
		_, err := record.NewSeq(0)
		require.Error(t, err)

		var vacia record.Seq
		assert.True(t, vacia.IsZero())
		assert.Empty(t, vacia.String())
	})
}

func TestEventType_ElVocabularioEsCerrado(t *testing.T) {
	conocidos := []record.EventType{
		record.TypeAttemptStarted,
		record.TypeStepStarted,
		record.TypeStepFinished,
		record.TypeCommandStarted,
		record.TypeCommandFinished,
		record.TypeParameterResolved,
		record.TypeArtifactProduced,
		record.TypeStaleCloneUsed,
		record.TypeSyncFailed,
		record.TypeAttemptFinished,
	}
	for _, tipo := range conocidos {
		assert.True(t, tipo.IsKnown(), "%s debería estar en el vocabulario", tipo)
		assert.NotEmpty(t, tipo.String())
	}

	assert.False(t, record.EventType("lo_que_sea").IsKnown())
	assert.False(t, record.EventType("").IsKnown())
}

func TestErrorClass_UnknownEsUnValorLegitimo(t *testing.T) {
	// Es la parte más valiosa de la disciplina: guardar hechos, nunca
	// conclusiones. Un fallo que no se sabe clasificar es exactamente eso.
	assert.True(t, record.ErrorClassUnknown.IsKnown())
	assert.False(t, record.ErrorClassUnknown.IsZero())

	assert.True(t, record.ErrorClassNone.IsZero())
	assert.True(t, record.ErrorClassNone.IsKnown())
	assert.Empty(t, record.ErrorClassNone.String())

	for _, clase := range []record.ErrorClass{
		record.ErrorClassCommandFailed,
		record.ErrorClassCancelled,
		record.ErrorClassInvalidPipelinecode,
		record.ErrorClassSourceUnavailable,
		record.ErrorClassStateUnavailable,
	} {
		assert.True(t, clase.IsKnown(), "%s debería estar en el vocabulario", clase)
	}

	assert.False(t, record.ErrorClass("lo_que_sea").IsKnown())
}
