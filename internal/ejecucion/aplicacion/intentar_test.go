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

	_, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.ErrorIs(t, err, publicado.ErrNoDisponible)
	require.Empty(t, d.historial.registros, "EJ-5: ningún paso empieza")
	require.Empty(t, d.comandos.llamados, "EJ-5: ningún comando corre")
	require.Equal(t, []string{"int-1"}, d.historial.abandonados,
		"el ambiente se reservó antes de tocar el espacio: si el espacio falla, el intento se abandona y el ambiente queda libre")
	require.False(t, d.historial.cerrado, "nunca llegó a ejecutar nada: no es un intento fallido")
}

func TestIntentar_UnAmbienteOcupadoNoTocaElEspacioDeTrabajo(t *testing.T) {
	// El espacio de trabajo del ambiente es de quien lo ocupa: si otro intento lo tiene, rehacerlo borraría los
	// directorios de los pasos del que está corriendo. Quien manda es el Historial, que decide quién ocupa el
	// ambiente, y por eso se le pregunta antes.
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.errAbrir = errors.New("ambiente ocupado")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.Error(t, err)
	require.False(t, d.espacioDeTrabajo.rehecho, "el espacio del intento que ocupa el ambiente quedó intacto")
	require.Empty(t, d.historial.abandonados, "no se abrió ningún intento que abandonar")
}

func TestIntentar_SiNoSePuedeAbandonarElIntentoQueNoEmpezoSeDiceAdemasDeLaCausa(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.espacioDeTrabajo.errRehacer = fmt.Errorf("disco lleno: %w", dominio.ErrNoDisponible)
	d.historial.errAbandonar = errors.New("el almacén no responde")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.ErrorIs(t, err, publicado.ErrNoDisponible, "la causa sigue siendo la del espacio")
	require.ErrorContains(t, err, "el almacén no responde", "y se dice que el ambiente pudo quedar ocupado")
}

func TestIntentar_UnaCopiaDeTrabajoAbreSinCommits(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.fuentes.material = dominio.Material{Directorio: "/tmp/wd", Hash: d.fuentes.material.Hash} // sin commit
	d.historial.despliegueACerrar = ""
	servicio := aplicacion.NuevoServicio(deps)

	peticion := peticionDePrueba()
	peticion.CopiaDeTrabajo = "/tmp/wd"

	resultado, err := servicio.Intentar(context.Background(), peticion)

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

	_, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.Error(t, err)
}

func TestIntentar_LaSalidaDeCadaComandoVaAlHistorial(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.NoError(t, err)
	require.NotEmpty(t, d.historial.salidas)
	primera := d.historial.salidas[0]
	require.Equal(t, "01-pruebas", primera.paso)
	require.NotEmpty(t, primera.comando)
	require.True(t, primera.exitoso)
	require.Contains(t, primera.texto, "salida de ")
}

func TestIntentar_LaSalidaDeUnComandoFallidoTambienVaAlHistorial(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.comandos.resultado = dominio.ResultadoDeUnComando{Exitoso: false}
	servicio := aplicacion.NuevoServicio(deps)

	_, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.NoError(t, err)
	require.Len(t, d.historial.salidas, 1, "se detiene en el primer comando que falla")
	require.False(t, d.historial.salidas[0].exitoso)
	require.Contains(t, d.historial.salidas[0].texto, "salida de ")
}

func TestIntentar_ElResultadoTraeElDetalleQueDaElHistorial(t *testing.T) {
	deps, d := nuevasDependenciasDePrueba(t, "01-pruebas")
	d.historial.detalle = dominio.DetalleDelIntento{
		Tiempo: 4 * time.Second,
		Pasos:  []dominio.PasoDelDetalle{{Nombre: "pruebas", Estado: dominio.PasoPrecargado}},
	}
	servicio := aplicacion.NuevoServicio(deps)

	resultado, err := servicio.Intentar(context.Background(), peticionDePrueba())

	require.NoError(t, err)
	require.Equal(t, "4s", resultado.Detalle.Tiempo)
	require.Equal(t, []publicado.PasoDelDetalle{{Nombre: "pruebas", Estado: "precargado"}}, resultado.Detalle.Pasos)
}
