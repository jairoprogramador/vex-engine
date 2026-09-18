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
	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/infraestructura"
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
		Reloj:        &relojQueAvanza{ahora: time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)},
		Identidades:  historialinfraestructura.IdentidadesUUID{},
	})
	return servicio, context.Background()
}

func aperturaDeUnSoloPaso(ambiente string) historialpublicado.Apertura {
	return historialpublicado.Apertura{
		Ambiente:      ambiente,
		Solicitante:   "ana",
		Pasos:         []historialpublicado.PasoDeclarado{{Nombre: "supply"}},
		HastaPaso:     "supply",
		ConCommits:    true,
		HashDelCodigo: "h1",
	}
}

func mustAmbitoProd(t *testing.T) dominio.Ambito {
	t.Helper()
	a, err := dominio.AmbitoDeAmbiente("prod")
	require.NoError(t, err)
	return a
}

var nada = historialpublicado.Contenido{}

func TestAdaptadorDeHistorial_RegistrarVariableEscribeElHashComoContenidoDeResolucion(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h, h)

	id, err := h.AbrirIntento(ctx, aperturaDeUnSoloPaso("prod"))
	require.NoError(t, err)
	require.NoError(t, h.RegistrarComienzo(ctx, id, "supply", nada))

	hash := dominio.CalcularHashDeVariable("un-valor")
	require.NoError(t, adaptador.RegistrarVariable(ctx, id, "supply", "n", hash, dominio.OrigenProducida, mustAmbitoProd(t)))
	require.NoError(t, h.RegistrarFinal(ctx, id, "supply", true, nada))

	variables, err := h.VariablesDeUnPaso(ctx, id, "supply")
	require.NoError(t, err)
	require.Len(t, variables, 1)
	require.Equal(t, "resolucion/variable-v1", variables[0].Contenido.Contexto)
	require.NotContains(t, string(variables[0].Contenido.Datos), "un-valor", "el hash nunca lleva el valor en claro")

	hashes, ok, err := adaptador.UltimaVezDeUnPaso(ctx, "supply", mustAmbitoProd(t))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, hash, hashes["n"])
}

func TestAdaptadorDeHistorial_UltimaVezDeUnPasoSigueElSaltoDeEvidencia(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h, h)
	prod := mustAmbitoProd(t)

	id1, err := h.AbrirIntento(ctx, aperturaDeUnSoloPaso("prod"))
	require.NoError(t, err)
	require.NoError(t, h.RegistrarComienzo(ctx, id1, "supply", nada))
	hash := dominio.CalcularHashDeVariable("un-valor")
	require.NoError(t, adaptador.RegistrarVariable(ctx, id1, "supply", "n", hash, dominio.OrigenProducida, prod))
	require.NoError(t, adaptador.GuardarValor(ctx, id1, "supply", "n", "un-valor"))
	require.NoError(t, h.RegistrarFinal(ctx, id1, "supply", true, nada))
	_, _, err = h.CerrarIntento(ctx, id1, historialpublicado.Exitoso, "")
	require.NoError(t, err)

	id2, err := h.AbrirIntento(ctx, aperturaDeUnSoloPaso("prod"))
	require.NoError(t, err)
	require.NoError(t, h.RegistrarNoReejecucion(
		ctx, id2, "supply", historialpublicado.Evidencia{Intento: id1, Paso: "supply"}, nada))

	hashes, ok, err := adaptador.UltimaVezDeUnPaso(ctx, "supply", prod)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, hash, hashes["n"], "el hash real vive en el intento que apunta la evidencia, no en la no re-ejecución")
}

func TestAdaptadorDeHistorial_ValoresDeLaUltimaVezUsaLaRelacionReservadaYConservaElAmbito(t *testing.T) {
	h, ctx := nuevoHistorialReal(t)
	adaptador := infraestructura.NuevoHistorial(h, h)
	prod := mustAmbitoProd(t)

	id1, err := h.AbrirIntento(ctx, aperturaDeUnSoloPaso("prod"))
	require.NoError(t, err)
	require.NoError(t, h.RegistrarComienzo(ctx, id1, "supply", nada))
	require.NoError(t, adaptador.RegistrarVariable(
		ctx, id1, "supply", "n", dominio.CalcularHashDeVariable("un-valor"), dominio.OrigenProducida, dominio.AmbitoCompartido()))
	require.NoError(t, adaptador.GuardarValor(ctx, id1, "supply", "n", "un-valor"))
	require.NoError(t, h.RegistrarFinal(ctx, id1, "supply", true, nada))
	_, _, err = h.CerrarIntento(ctx, id1, historialpublicado.Exitoso, "")
	require.NoError(t, err)

	id2, err := h.AbrirIntento(ctx, aperturaDeUnSoloPaso("prod"))
	require.NoError(t, err)
	require.NoError(t, h.RegistrarNoReejecucion(
		ctx, id2, "supply", historialpublicado.Evidencia{Intento: id1, Paso: "supply"}, nada))

	valores, ok, err := adaptador.ValoresDeLaUltimaVez(ctx, "supply", prod)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "un-valor", valores["n"].Valor)
	require.Equal(t, dominio.AmbitoCompartido(), valores["n"].Ambito, "conserva el ámbito con el que se produjo, no el que se preguntó")
}
