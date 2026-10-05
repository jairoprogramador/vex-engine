package borde_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/borde"
	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// ejecucionFalsa es el doble del único puerto que declara internal/borde (ejecucionpublicado.ParaBorde): las
// pruebas de punta a punta ejercitan el borde contra el Servicio real de Ejecución.
type ejecucionFalsa struct {
	resultado          ejecucionpublicado.Resultado
	err                error
	peticionDeIntento  ejecucionpublicado.PeticionDeIntento
	peticionDeRollback ejecucionpublicado.PeticionDeRollback
	entorno            ejecucionpublicado.Entorno
	llamado            string
}

func (e *ejecucionFalsa) Intentar(
	_ context.Context, p ejecucionpublicado.PeticionDeIntento, entorno ejecucionpublicado.Entorno,
) (ejecucionpublicado.Resultado, error) {
	e.llamado = "intentar"
	e.peticionDeIntento = p
	e.entorno = entorno
	return e.resultado, e.err
}

func (e *ejecucionFalsa) HacerRollback(
	_ context.Context, p ejecucionpublicado.PeticionDeRollback, entorno ejecucionpublicado.Entorno,
) (ejecucionpublicado.Resultado, error) {
	e.llamado = "rollback"
	e.peticionDeRollback = p
	e.entorno = entorno
	return e.resultado, e.err
}

var _ ejecucionpublicado.ParaBorde = (*ejecucionFalsa)(nil)

func TestServicio_IntentarRechazaUnaVersionNoSoportadaSinLlamarAEjecucion(t *testing.T) {
	falsa := &ejecucionFalsa{}
	servicio := borde.NuevoServicio(borde.Dependencias{Ejecucion: falsa})

	_, err := servicio.Intentar(context.Background(), ejecucionpublicado.PeticionDeIntento{Version: "99"}, nil)

	require.ErrorIs(t, err, borde.ErrVersionNoSoportada)
	require.Empty(t, falsa.llamado)
}

func TestServicio_IntentarDelegaEnEjecucionConUnaVersionSoportada(t *testing.T) {
	falsa := &ejecucionFalsa{resultado: ejecucionpublicado.Resultado{Intento: "int-1", Estado: "exitoso"}}
	servicio := borde.NuevoServicio(borde.Dependencias{Ejecucion: falsa})

	resultado, err := servicio.Intentar(context.Background(), ejecucionpublicado.PeticionDeIntento{Version: "1", Ambiente: "prod"}, nil)

	require.NoError(t, err)
	require.Equal(t, "intentar", falsa.llamado)
	require.Equal(t, "prod", falsa.peticionDeIntento.Ambiente)
	require.Equal(t, "int-1", resultado.Intento)
}

func TestServicio_HacerRollbackRechazaUnaVersionNoSoportadaSinLlamarAEjecucion(t *testing.T) {
	falsa := &ejecucionFalsa{}
	servicio := borde.NuevoServicio(borde.Dependencias{Ejecucion: falsa})

	_, err := servicio.HacerRollback(context.Background(), ejecucionpublicado.PeticionDeRollback{Version: "99"}, nil)

	require.ErrorIs(t, err, borde.ErrVersionNoSoportada)
	require.Empty(t, falsa.llamado)
}

func TestServicio_HacerRollbackDelegaEnEjecucionConUnaVersionSoportada(t *testing.T) {
	falsa := &ejecucionFalsa{resultado: ejecucionpublicado.Resultado{Despliegue: "dep-2"}}
	servicio := borde.NuevoServicio(borde.Dependencias{Ejecucion: falsa})

	resultado, err := servicio.HacerRollback(
		context.Background(), ejecucionpublicado.PeticionDeRollback{Version: "1", Despliegue: "dep-1"}, nil,
	)

	require.NoError(t, err)
	require.Equal(t, "rollback", falsa.llamado)
	require.Equal(t, "dep-1", falsa.peticionDeRollback.Despliegue)
	require.Equal(t, "dep-2", resultado.Despliegue)
}

func TestServicio_ElEntornoLlegaAEjecucionTalCualYNoSeToca(t *testing.T) {
	falsa := &ejecucionFalsa{}
	servicio := borde.NuevoServicio(borde.Dependencias{Ejecucion: falsa})
	entorno := ejecucionpublicado.Entorno{"A": "1"}

	_, err := servicio.Intentar(context.Background(), ejecucionpublicado.PeticionDeIntento{Version: "1"}, entorno)
	require.NoError(t, err)
	require.Equal(t, entorno, falsa.entorno)

	_, err = servicio.HacerRollback(context.Background(), ejecucionpublicado.PeticionDeRollback{Version: "1"}, entorno)
	require.NoError(t, err)
	require.Equal(t, entorno, falsa.entorno)
}
