package aplicacion_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

func destinoDePrueba(t *testing.T) dominio.Destino {
	t.Helper()
	d, err := dominio.NuevoDestino("dep-1", "prod", "git@proyecto", "c-viejo-proyecto", "git@pipeline", "c-viejo-pipeline")
	require.NoError(t, err)
	return d
}

func TestHacerRollback_UsaElMaterialYElPipelineDelDestino(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	d.historial.destinoParaRollback = destinoDePrueba(t)
	d.historial.despliegueACerrar = "dep-2"
	servicio := aplicacion.NuevoServicio(deps)

	resultado, err := servicio.HacerRollback(context.Background(), publicado.PeticionDeRollback{
		Version: "1", Despliegue: "dep-1", Solicitante: "ana",
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "dep-2", resultado.Despliegue)
	require.Len(t, d.historial.aperturas, 1)
	apertura := d.historial.aperturas[0]
	require.Equal(t, "prod", apertura.Ambiente)
	require.Equal(t, "git@proyecto", apertura.FuenteDelProyecto)
	require.Equal(t, "c-viejo-proyecto", apertura.CommitDelProyecto)
	require.Equal(t, "git@pipeline", apertura.FuenteDelPipeline)
	require.Equal(t, "c-viejo-pipeline", apertura.CommitDelPipeline)
	require.True(t, apertura.ConCommits)
	require.Equal(t, "02-despliegue", apertura.HastaPaso, "EJ-2 hace todos los pasos del pipeline, hasta el último")
	require.Len(t, apertura.Pasos, 2)
}

func TestHacerRollback_UnAmbienteOcupadoNoTocaElEspacioDeTrabajo(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.destinoParaRollback = destinoDePrueba(t)
	d.historial.errAbrir = errors.New("ambiente ocupado")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.HacerRollback(context.Background(), publicado.PeticionDeRollback{
		Version: "1", Despliegue: "dep-1", Solicitante: "ana",
	}, nil)

	require.Error(t, err)
	require.False(t, d.espacioDeTrabajo.rehecho, "el espacio del intento que ocupa el ambiente quedó intacto")
}

func TestHacerRollback_SiElEspacioDeTrabajoNoEstaDisponibleElIntentoSeAbandona(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.destinoParaRollback = destinoDePrueba(t)
	d.espacioDeTrabajo.errRehacer = fmt.Errorf("disco lleno: %w", dominio.ErrNoDisponible)
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.HacerRollback(context.Background(), publicado.PeticionDeRollback{
		Version: "1", Despliegue: "dep-1", Solicitante: "ana",
	}, nil)

	require.ErrorIs(t, err, publicado.ErrNoDisponible)
	require.Equal(t, []string{"int-1"}, d.historial.abandonados)
	require.Empty(t, d.historial.registros)
}

func TestHacerRollback_CierraConElDestinoComoPadre(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.destinoParaRollback = destinoDePrueba(t)
	d.historial.despliegueACerrar = "dep-2"
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.HacerRollback(context.Background(), publicado.PeticionDeRollback{
		Version: "1", Despliegue: "dep-1", Solicitante: "ana",
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "dep-1", d.historial.destinoCerrado)
}

func TestHacerRollback_PropagaElErrorSiElDestinoNoSePuedeResolver(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.errDestinoParaRollback = errors.New("no se pudo resolver el despliegue")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.HacerRollback(context.Background(), publicado.PeticionDeRollback{
		Version: "1", Despliegue: "dep-1", Solicitante: "ana",
	}, nil)

	require.Error(t, err)
	require.Empty(t, d.historial.aperturas)
}
