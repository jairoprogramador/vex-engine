package aplicacion_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

func TestIntentar_SiElEspacioDeTrabajoNoEstaDisponibleElIntentoNoEmpieza(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.espacioDeTrabajo.errRehacer = fmt.Errorf("disco lleno: %w", dominio.ErrNoDisponible)
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), &salidaFalsa{})

	require.Error(t, err)
	require.Empty(t, d.historial.aperturas, "EJ-5: el intento no debe llegar a abrirse si el espacio de trabajo no está disponible")
}

func TestIntentar_UnaCopiaDeTrabajoAbreSinCommits(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.fuentes.material = dominio.Material{Directorio: "/tmp/wd", Hash: d.fuentes.material.Hash} // sin commit
	d.historial.despliegueACerrar = ""
	servicio := aplicacion.NuevoServicio(deps)

	peticion := peticionDePrueba()
	peticion.CopiaDeTrabajo = "/tmp/wd"

	resultado, err := servicio.Intentar(context.Background(), peticion, &salidaFalsa{})

	require.NoError(t, err)
	require.Len(t, d.historial.aperturas, 1)
	require.False(t, d.historial.aperturas[0].ConCommits, "DEC-10.7: una copia de trabajo nunca declara commits")
	require.Empty(t, resultado.Despliegue)
	require.Len(t, d.fuentes.retirados, 1, "el material se retira aunque el intento termine bien")
}

func TestIntentar_ElSegundoIntentoEnElMismoAmbienteSeRechaza(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.errAbrir = errors.New("ambiente ocupado")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), &salidaFalsa{})

	require.Error(t, err)
}

func TestIntentar_LaSalidaLlegaPorCadaPasoMientrasCorre(t *testing.T) {
	deps, _ := nuevasDependenciasDePrueba(t, "01-pruebas")
	servicio := aplicacion.NuevoServicio(deps)
	salida := &salidaFalsa{}

	_, err := servicio.Intentar(context.Background(), peticionDePrueba(), salida)

	require.NoError(t, err)
	require.NotEmpty(t, salida.recibido)
	require.Contains(t, salida.recibido[0], "01-pruebas:")
}

func TestIntentar_ElResultadoTraeElDetalleQueDaElHistorial(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.detalle = dominio.DetalleDelIntento{
		Tiempo: 4 * time.Second,
		Pasos:  []dominio.PasoDelDetalle{{Nombre: "pruebas", Estado: dominio.PasoPrecargado}},
	}
	servicio := aplicacion.NuevoServicio(deps)

	resultado, err := servicio.Intentar(context.Background(), peticionDePrueba(), &salidaFalsa{})

	require.NoError(t, err)
	require.Equal(t, "4s", resultado.Detalle.Tiempo)
	require.Equal(t, []publicado.PasoDelDetalle{{Nombre: "pruebas", Estado: "precargado"}}, resultado.Detalle.Pasos)
}
