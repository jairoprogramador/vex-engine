package infraestructura_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	historialaplicacion "github.com/jairoprogramador/vex-engine/internal/historial/aplicacion"
	historialinfraestructura "github.com/jairoprogramador/vex-engine/internal/historial/infraestructura"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/infraestructura"
)

// El adaptador se prueba contra el Historial real, sobre su almacén en memoria (DEC-11.3): es la frontera de
// otro contexto, y aquí solo se comprueba la traducción — nunca el propio Historial.

type relojQueAvanza struct {
	mu    sync.Mutex
	ahora time.Time
}

func (r *relojQueAvanza) Ahora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ahora = r.ahora.Add(time.Second)
	return r.ahora
}

func nuevoHistorialReal(t *testing.T) (*historialaplicacion.Servicio, context.Context) {
	t.Helper()
	almacen := historialinfraestructura.NuevoAlmacenEnMemoria()
	servicio := historialaplicacion.NuevoServicio(historialaplicacion.Dependencias{
		Intentos:     historialinfraestructura.NuevosIntentos(almacen),
		Despliegues:  historialinfraestructura.NuevosDespliegues(almacen),
		Ocupaciones:  historialinfraestructura.NuevasOcupaciones(almacen),
		Lanzamientos: historialinfraestructura.NuevosLanzamientos(almacen),
		Reservas:     historialinfraestructura.NuevasReservas(almacen),
		Salidas:      historialinfraestructura.NuevasSalidas(almacen),
		Reloj:        &relojQueAvanza{ahora: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)},
		Identidades:  historialinfraestructura.IdentidadesUUID{},
	})
	return servicio, context.Background()
}

var nada = historialpublicado.Contenido{}

// desplegar abre un intento de un solo paso, lo hace bien y lo cierra, con el hash de código dado.
func desplegar(t *testing.T, h *historialaplicacion.Servicio, ctx context.Context, ambiente, hash string) string {
	t.Helper()
	id, err := h.AbrirIntento(ctx, historialpublicado.Apertura{
		Ambiente: ambiente, Solicitante: "ana", Pasos: []historialpublicado.PasoDeclarado{{Nombre: "deploy"}},
		HastaPaso: "deploy", ConCommits: true, HashDelCodigo: hash,
	})
	require.NoError(t, err)
	require.NoError(t, h.RegistrarComienzo(ctx, id, "deploy", nada))
	require.NoError(t, h.RegistrarFinal(ctx, id, "deploy", true, nada))
	d, hay, err := h.CerrarIntento(ctx, id, historialpublicado.Exitoso, "", "")
	require.NoError(t, err)
	require.True(t, hay)
	return d.Id
}

func mustAmbiente(t *testing.T, valor string) dominio.Ambiente {
	t.Helper()
	a, err := dominio.NuevaAmbiente(valor)
	require.NoError(t, err)
	return a
}

func mustIdDespliegue(t *testing.T, valor string) dominio.IdDespliegue {
	t.Helper()
	d, err := dominio.NuevoIdDespliegue(valor)
	require.NoError(t, err)
	return d
}

func mustHash(t *testing.T, valor string) dominio.HashDelCodigo {
	t.Helper()
	h, err := dominio.NuevoHashDelCodigo(valor)
	require.NoError(t, err)
	return h
}

func mustVersion(t *testing.T, numero int) dominio.Version {
	t.Helper()
	v, err := dominio.NuevaVersion(numero)
	require.NoError(t, err)
	return v
}

func TestAdaptadorDeHistorial_RegistrarLanzamientoCodificaLaVersionElNombreYElHash(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)
	despliegue := desplegar(t, h, ctx, "staging", "h1")

	lanzamiento := dominio.NuevoLanzamiento(
		mustIdDespliegue(t, despliegue), mustHash(t, "h1"), mustVersion(t, 1), "listo-para-el-cliente")
	registrado, err := adaptador.RegistrarLanzamiento(ctx, mustAmbiente(t, "staging"), lanzamiento)
	require.NoError(t, err)
	require.Equal(t, "staging", registrado.Ambiente.String())
	require.Equal(t, despliegue, registrado.Despliegue.String())
	require.Equal(t, 1, registrado.Version.Numero())
	require.Equal(t, "listo-para-el-cliente", registrado.Nombre.String())
	require.NotZero(t, registrado.Instante)

	guardado, ok, err := h.UltimoLanzamiento(ctx, "staging")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "lanzamiento/lanzamiento-v1", guardado.Contenido.Contexto)
	require.Contains(t, string(guardado.Contenido.Datos), `"nombre":"listo-para-el-cliente"`)
}

func TestAdaptadorDeHistorial_VersionesConocidasEsPorProyectoNoPorAmbiente(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)
	d1 := desplegar(t, h, ctx, "staging", "h1")
	d2 := desplegar(t, h, ctx, "prod", "h2")

	_, err := adaptador.RegistrarLanzamiento(
		ctx, mustAmbiente(t, "staging"), dominio.NuevoLanzamiento(mustIdDespliegue(t, d1), mustHash(t, "h1"), mustVersion(t, 1), ""))
	require.NoError(t, err)
	_, err = adaptador.RegistrarLanzamiento(
		ctx, mustAmbiente(t, "prod"), dominio.NuevoLanzamiento(mustIdDespliegue(t, d2), mustHash(t, "h2"), mustVersion(t, 2), ""))
	require.NoError(t, err)

	conocidas, err := adaptador.VersionesConocidas(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []dominio.VersionConocida{
		{Hash: mustHash(t, "h1"), Version: mustVersion(t, 1)},
		{Hash: mustHash(t, "h2"), Version: mustVersion(t, 2)},
	}, conocidas)
}

func TestAdaptadorDeHistorial_ReservadoYUltimoLanzamientoDeUnAmbiente(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)

	reservado, err := adaptador.Reservado(ctx, mustAmbiente(t, "prod"))
	require.NoError(t, err)
	require.False(t, reservado, "sin ninguna reserva registrada, no está reservado")

	_, hay, err := adaptador.UltimoLanzamientoDeUnAmbiente(ctx, mustAmbiente(t, "prod"))
	require.NoError(t, err)
	require.False(t, hay)

	require.NoError(t, adaptador.RegistrarReserva(ctx, mustAmbiente(t, "prod"), true))
	reservado, err = adaptador.Reservado(ctx, mustAmbiente(t, "prod"))
	require.NoError(t, err)
	require.True(t, reservado)

	d := desplegar(t, h, ctx, "prod", "h1")
	_, err = adaptador.RegistrarLanzamiento(
		ctx, mustAmbiente(t, "prod"), dominio.NuevoLanzamiento(mustIdDespliegue(t, d), mustHash(t, "h1"), mustVersion(t, 1), ""))
	require.NoError(t, err)

	ultimo, hay, err := adaptador.UltimoLanzamientoDeUnAmbiente(ctx, mustAmbiente(t, "prod"))
	require.NoError(t, err)
	require.True(t, hay)
	require.Equal(t, d, ultimo.String())
}

func TestAdaptadorDeHistorial_HashDelCodigoDeUnDespliegue(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)
	d := desplegar(t, h, ctx, "staging", "h1")

	hash, err := adaptador.HashDelCodigoDeUnDespliegue(ctx, mustIdDespliegue(t, d))

	require.NoError(t, err)
	require.Equal(t, "h1", hash.String())
}

func TestAdaptadorDeHistorial_LanzamientosDeUnAmbienteFiltraYDecodificaVersionYNombre(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h)
	d1 := desplegar(t, h, ctx, "staging", "h1")
	d2 := desplegar(t, h, ctx, "prod", "h2")
	primero, err := adaptador.RegistrarLanzamiento(
		ctx, mustAmbiente(t, "staging"), dominio.NuevoLanzamiento(mustIdDespliegue(t, d1), mustHash(t, "h1"), mustVersion(t, 1), "uno"))
	require.NoError(t, err)
	_, err = adaptador.RegistrarLanzamiento(
		ctx, mustAmbiente(t, "prod"), dominio.NuevoLanzamiento(mustIdDespliegue(t, d2), mustHash(t, "h2"), mustVersion(t, 2), "dos"))
	require.NoError(t, err)

	lanzamientos, err := adaptador.LanzamientosDeUnAmbiente(ctx, mustAmbiente(t, "staging"))

	require.NoError(t, err)
	require.Len(t, lanzamientos, 1)
	require.Equal(t, primero.Id, lanzamientos[0].Id)
	require.NotEmpty(t, lanzamientos[0].Id)
	require.Equal(t, d1, lanzamientos[0].Despliegue.String())
	require.Equal(t, 1, lanzamientos[0].Version.Numero())
	require.Equal(t, "uno", lanzamientos[0].Nombre.String())
	require.Equal(t, primero.Instante, lanzamientos[0].Instante)
}
