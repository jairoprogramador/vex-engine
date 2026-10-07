package infraestructura_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
	suministropublicado "github.com/jairoprogramador/vex-engine/internal/suministro/publicado"
)

func campoYValor(err error) (campo, valor string, ok bool) {
	var parametro interface{ ParametroInvalido() (string, string) }
	if !errors.As(err, &parametro) {
		return "", "", false
	}
	campo, valor = parametro.ParametroInvalido()
	return campo, valor, true
}

// Solo dice el campo cuando el adaptador sabe de qué campo de la petición viene lo rechazado: una fuente que no
// está («hoy»), o una copia de trabajo. Con un commit no se sabe si falla la fuente o el commit, y no se adivina.
func TestParametroInvalido_ElRechazoDeArribaDiceElCampoSoloCuandoElAdaptadorLoSabe(t *testing.T) {
	casos := map[string]struct {
		llamar       func() error
		campo, valor string
		conCampo     bool
	}{
		"el pipeline de hoy": {func() error {
			_, err := infraestructura.NuevosPipelines(&pipelinesFalsos{err: definicionpublicado.ErrNoExiste}).
				DeHoy(context.Background(), "/no/existe")
			return err
		}, "FuenteDelPipeline", "/no/existe", true},
		"el material de hoy": {func() error {
			_, err := infraestructura.NuevasFuentes(&fuentesFalsas{err: suministropublicado.ErrNoExiste}).
				TraerDeHoy(context.Background(), "/no/existe")
			return err
		}, "FuenteDelProyecto", "/no/existe", true},
		"la copia de trabajo": {func() error {
			_, err := infraestructura.NuevasFuentes(&fuentesFalsas{err: suministropublicado.ErrNoExiste}).
				TraerCopiaDeTrabajo(context.Background(), "/no/hay")
			return err
		}, "CopiaDeTrabajo", "/no/hay", true},
		"el pipeline de un commit: puede ser la fuente o el commit": {func() error {
			_, err := infraestructura.NuevosPipelines(&pipelinesFalsos{err: definicionpublicado.ErrNoExiste}).
				DeUnCommit(context.Background(), "f", "c")
			return err
		}, "", "", false},
		"el material de un commit: puede ser la fuente o el commit": {func() error {
			_, err := infraestructura.NuevasFuentes(&fuentesFalsas{err: suministropublicado.ErrNoExiste}).
				TraerDeUnCommit(context.Background(), "f", "c")
			return err
		}, "", "", false},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			err := c.llamar()

			require.ErrorIs(t, err, dominio.ErrInvalido)
			campo, valor, ok := campoYValor(err)
			require.True(t, ok, "errorDeArriba siempre tiene el método")
			require.Equal(t, c.campo, campo)
			require.Equal(t, c.valor, valor)
			if !c.conCampo {
				require.Empty(t, campo, "sin adivinar")
			}
		})
	}
}

func TestParametroInvalido_UnaFuenteVaciaDiceDeQueCampoViene(t *testing.T) {
	e := infraestructura.NuevoEspacioDeTrabajo(t.TempDir())

	_, errProyecto := e.Ubicar("", "/pipeline", "prod")
	_, errPipeline := e.Ubicar("/proyecto", "", "prod")

	for esperado, err := range map[string]error{"FuenteDelProyecto": errProyecto, "FuenteDelPipeline": errPipeline} {
		require.ErrorIs(t, err, dominio.ErrInvalido)
		campo, valor, ok := campoYValor(err)
		require.True(t, ok)
		require.Equal(t, esperado, campo)
		require.Empty(t, valor)
	}
}
