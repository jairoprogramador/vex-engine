package infraestructura_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/infraestructura"
)

func campoYValor(err error) (campo, valor string) {
	var parametro interface{ ParametroInvalido() (string, string) }
	if errors.As(err, &parametro) {
		campo, valor = parametro.ParametroInvalido()
	}
	return campo, valor
}

func TestParametroInvalido_LaCopiaDeTrabajoQueNoEstaDiceSuCampoYUnCommitNoAdivina(t *testing.T) {
	adaptador := infraestructura.NuevosPipelines(&pipelinesFalsos{err: definicionpublicado.ErrNoExiste})

	_, errCopia := adaptador.DeUnaCopiaDeTrabajo(context.Background(), "/no/hay")
	_, errCommit := adaptador.DeUnCommit(context.Background(), "f", "c")

	require.ErrorIs(t, errCopia, dominio.ErrInvalido)
	campo, valor := campoYValor(errCopia)
	require.Equal(t, "CopiaDeTrabajo", campo)
	require.Equal(t, "/no/hay", valor)

	require.ErrorIs(t, errCommit, dominio.ErrInvalido)
	campo, _ = campoYValor(errCommit)
	require.Empty(t, campo, "la fuente o el commit: no se sabe cuál")
}
