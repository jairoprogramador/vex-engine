package infraestructura_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
	resolucionpublicado "github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
	suministropublicado "github.com/jairoprogramador/vex-engine/internal/suministro/publicado"
)

// Los errores de los contextos de arriba entran al dominio de Ejecución como ErrInvalido o ErrRechazado, para que
// el borde los distinga de un fallo del motor; y siguen siendo el error concreto que los causó.

func fallosDeComprobacion() error {
	return definicionpublicado.NuevosFallosDeComprobacion(
		[]definicionpublicado.Fallo{{Invariante: "I-1", Fichero: "config.yaml", Detalle: "falta"}}, "")
}

func TestErroresDeArriba_LoQueSeApuntaYNoEstaEsInvalido(t *testing.T) {
	casos := map[string]struct {
		err    error
		llamar func(error) error
	}{
		"el pipeline no existe": {definicionpublicado.ErrNoExiste, func(e error) error {
			_, err := infraestructura.NuevosPipelines(&pipelinesFalsos{err: e}).DeHoy(context.Background(), "f")
			return err
		}},
		"el commit del pipeline no existe": {definicionpublicado.ErrNoExiste, func(e error) error {
			_, err := infraestructura.NuevosPipelines(&pipelinesFalsos{err: e}).DeUnCommit(context.Background(), "f", "c")
			return err
		}},
		"el pipeline no se puede pedir": {definicionpublicado.ErrInvalido, func(e error) error {
			_, err := infraestructura.NuevosPipelines(&pipelinesFalsos{err: e}).DeUnCommit(context.Background(), "f", "c")
			return err
		}},
		"la fuente no existe": {suministropublicado.ErrNoExiste, func(e error) error {
			_, err := infraestructura.NuevasFuentes(&fuentesFalsas{err: e}).TraerDeHoy(context.Background(), "f")
			return err
		}},
		"el commit de la fuente no existe": {suministropublicado.ErrNoExiste, func(e error) error {
			_, err := infraestructura.NuevasFuentes(&fuentesFalsas{err: e}).TraerDeUnCommit(context.Background(), "f", "c")
			return err
		}},
		"la fuente no se puede pedir": {suministropublicado.ErrInvalido, func(e error) error {
			_, err := infraestructura.NuevasFuentes(&fuentesFalsas{err: e}).TraerDeHoy(context.Background(), "f")
			return err
		}},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			err := c.llamar(c.err)

			require.ErrorIs(t, err, dominio.ErrInvalido)
			require.NotErrorIs(t, err, dominio.ErrRechazado)
			require.ErrorIs(t, err, c.err, "sigue siendo el error de quien lo causó")
		})
	}
}

func TestErroresDeArriba_UnPipelineQueNoPasaLaComprobacionEsRechazadoYConservaSusFallos(t *testing.T) {
	_, err := infraestructura.NuevosPipelines(&pipelinesFalsos{err: fallosDeComprobacion()}).DeHoy(context.Background(), "f")

	require.ErrorIs(t, err, dominio.ErrRechazado)
	require.ErrorIs(t, err, definicionpublicado.ErrNoComprobado)
	var fallos *definicionpublicado.FallosDeComprobacion
	require.ErrorAs(t, err, &fallos, "los fallos concretos siguen al alcance")
	require.Equal(t, "config.yaml", fallos.Fallos[0].Fichero)
}

func TestErroresDeArriba_UnaVariableQueNoExisteEsRechazadaYDiceCual(t *testing.T) {
	adaptador := infraestructura.NuevasVariables(&variablesFalsas{err: &resolucionpublicado.VariableNoEncontradaError{Nombre: "deploy"}})
	ambito, err := dominio.AmbitoDeAmbiente("prod")
	require.NoError(t, err)

	_, err = adaptador.Interpolar(context.Background(), "i", "p", ambito, "${var.deploy}")

	require.ErrorIs(t, err, dominio.ErrRechazado)
	var falta *resolucionpublicado.VariableNoEncontradaError
	require.ErrorAs(t, err, &falta)
	require.Equal(t, "deploy", falta.Nombre)
}

func TestErroresDeArriba_LoQueNoSeReconoceSigueSiendoUnFallo(t *testing.T) {
	roto := errors.New("el disco se llenó")
	_, errPipeline := infraestructura.NuevosPipelines(&pipelinesFalsos{err: roto}).DeHoy(context.Background(), "f")
	_, errFuente := infraestructura.NuevasFuentes(&fuentesFalsas{err: roto}).TraerDeHoy(context.Background(), "f")

	for _, err := range []error{errPipeline, errFuente} {
		require.ErrorIs(t, err, roto)
		require.NotErrorIs(t, err, dominio.ErrInvalido)
		require.NotErrorIs(t, err, dominio.ErrRechazado)
	}
}

func TestErroresDeArriba_ElMensajeNoCambia(t *testing.T) {
	_, err := infraestructura.NuevasFuentes(&fuentesFalsas{err: suministropublicado.ErrNoExiste}).TraerDeHoy(context.Background(), "/no/existe")

	require.Equal(t, `ejecución: el material de "/no/existe": suministro: no existe`, err.Error())
}
