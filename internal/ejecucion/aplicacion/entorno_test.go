package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

func listas(d *dependenciasDePrueba) [][]string {
	listas := make([][]string, 0, len(d.comandos.entornos))
	for _, e := range d.comandos.entornos {
		listas = append(listas, e.Lista())
	}
	return listas
}

func TestEntorno_LlegaACadaComandoDeCadaPaso(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas", "02-despliegue")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), publicado.Entorno{"REGISTRY_URL": "registry.local", "A": "1"})

	require.NoError(t, err)
	require.Equal(t, [][]string{
		{"A=1", "REGISTRY_URL=registry.local"},
		{"A=1", "REGISTRY_URL=registry.local"},
	}, listas(d), "los dos comandos, uno por paso, lo reciben")
}

func TestEntorno_SinVariablesLosComandosNoRecibenNinguna(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), nil)

	require.NoError(t, err)
	require.Equal(t, [][]string{{}}, listas(d))
}

func TestEntorno_UnNombreInvalidoSeRechazaAntesDeAbrirNiTraerNada(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), publicado.Entorno{"NOMBRE-MALO": "valor-sensible-123"})

	require.ErrorIs(t, err, publicado.ErrInvalido)
	require.ErrorContains(t, err, "NOMBRE-MALO")
	require.NotContains(t, err.Error(), "valor-sensible-123", "el error dice la variable, no su valor")
	require.Empty(t, d.historial.aperturas, "no se abrió ningún intento")
	require.Empty(t, d.fuentes.retirados, "ni se trajo material")
	require.False(t, d.espacioDeTrabajo.rehecho, "ni se tocó el espacio de trabajo")
	require.Empty(t, d.comandos.llamados)
}

func TestEntorno_UnRollbackTambienLoPasaALosComandos(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.destinoParaRollback = destinoDePrueba(t)
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.HacerRollback(context.Background(), publicado.PeticionDeRollback{
		Version: "1", Despliegue: "dep-1", Solicitante: "ana",
	}, publicado.Entorno{"A": "1"})

	require.NoError(t, err)
	require.Equal(t, [][]string{{"A=1"}}, listas(d))
}

func TestEntorno_UnRollbackConUnNombreInvalidoSeRechazaSinAbrirNada(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.destinoParaRollback = destinoDePrueba(t)
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.HacerRollback(context.Background(), publicado.PeticionDeRollback{
		Version: "1", Despliegue: "dep-1", Solicitante: "ana",
	}, publicado.Entorno{"1MALO": "x"})

	require.ErrorIs(t, err, publicado.ErrInvalido)
	require.Empty(t, d.historial.aperturas)
}
