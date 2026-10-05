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

// Lo que Definición dice de un pipeline que no está donde se dice entra al dominio de Simulación como ErrInvalido,
// y sigue siendo el error concreto que lo causó.

func TestErroresDeArriba_UnPipelineQueNoExisteOQueNoSePuedePedirEsInvalido(t *testing.T) {
	for _, causa := range []error{definicionpublicado.ErrNoExiste, definicionpublicado.ErrInvalido} {
		t.Run(causa.Error(), func(t *testing.T) {
			adaptador := infraestructura.NuevosPipelines(&pipelinesFalsos{err: causa})

			_, errCommit := adaptador.DeUnCommit(context.Background(), "f", "c")
			_, errCopia := adaptador.DeUnaCopiaDeTrabajo(context.Background(), "/d")

			for _, err := range []error{errCommit, errCopia} {
				require.ErrorIs(t, err, dominio.ErrInvalido)
				require.ErrorIs(t, err, causa)
			}
		})
	}
}

func TestErroresDeArriba_UnPipelineQueNoPasaLaComprobacionSigueSiendoSusFallos(t *testing.T) {
	fallos := definicionpublicado.NuevosFallosDeComprobacion(
		[]definicionpublicado.Fallo{{Invariante: "formato", Fichero: "config.yaml", Detalle: "no se lee"}}, "")
	adaptador := infraestructura.NuevosPipelines(&pipelinesFalsos{err: fallos})

	_, err := adaptador.DeUnCommit(context.Background(), "f", "c")

	var recibidos *definicionpublicado.FallosDeComprobacion
	require.ErrorAs(t, err, &recibidos, "la aplicación lo reconoce y lo convierte en el resultado de la simulación")
	require.NotErrorIs(t, err, dominio.ErrInvalido, "no es un error de la petición")
}

func TestErroresDeArriba_LoQueNoSeReconoceSigueSiendoUnFallo(t *testing.T) {
	roto := errors.New("el disco se llenó")
	adaptador := infraestructura.NuevosPipelines(&pipelinesFalsos{err: roto})

	_, err := adaptador.DeUnCommit(context.Background(), "f", "c")

	require.ErrorIs(t, err, roto)
	require.NotErrorIs(t, err, dominio.ErrInvalido)
}
