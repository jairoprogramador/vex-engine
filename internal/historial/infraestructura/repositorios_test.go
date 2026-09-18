package infraestructura

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

var t0 = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

func en(minuto int) time.Time { return t0.Add(time.Duration(minuto) * time.Minute) }

func aperturaDePrueba() dominio.Apertura {
	return dominio.Apertura{
		Ambiente:      "staging",
		Solicitante:   "ana",
		Pasos:         []dominio.PasoDeclarado{{Nombre: "test"}, {Nombre: "supply", Compartido: true}, {Nombre: "deploy"}},
		HastaPaso:     "deploy",
		ConCommits:    true,
		HashDelCodigo: "h1",
	}
}

func TestIntentosEnAlmacen_GuardanYLeenCadaTipoDeRegistro(t *testing.T) {
	const secreto = "s3cr3t-de-produccion"
	raiz, ctx := t.TempDir(), contexto()
	almacen, err := NuevoAlmacenLocal(raiz)
	require.NoError(t, err)
	intentos := NuevosIntentos(almacen)
	nada := dominio.Contenido{}

	previo := dominio.NuevoIntento("i0")
	require.NoError(t, previo.Abrir(aperturaDePrueba(), en(0), nada))
	require.NoError(t, previo.Comenzar("supply", en(1), nada))
	require.NoError(t, previo.Terminar("supply", true, en(2), nada))
	require.NoError(t, intentos.Anadir(ctx, previo))

	i := dominio.NuevoIntento("i1")
	require.NoError(t, i.Abrir(aperturaDePrueba(), en(3),
		dominio.Contenido{Contexto: "suministro", Datos: []byte(`{"commit":"abc"}`)}))
	require.NoError(t, i.Comenzar("test", en(4), nada))
	require.NoError(t, i.RegistrarVariable("test", "DB_PASSWORD", en(5),
		dominio.Contenido{Contexto: "resolucion", Datos: []byte(`{"hash":"x"}`)}))
	require.NoError(t, i.GuardarValor("test", "DB_PASSWORD", secreto, en(5)))
	require.NoError(t, i.Terminar("test", true, en(6), nada))
	require.NoError(t, i.NoReejecutar("supply", dominio.Evidencia{Intento: "i0", Paso: "supply"}, previo, en(7),
		dominio.Contenido{Contexto: "ejecucion", Datos: []byte(`{"razon":"nada cambió"}`)}))
	require.NoError(t, i.Comenzar("deploy", en(8), nada))
	require.NoError(t, i.Terminar("deploy", false, en(9), nada))
	require.NoError(t, i.Cerrar(dominio.Cierre{Estado: dominio.Fallido}, en(10),
		dominio.NuevosDesplieguesDeUnAmbiente("staging")))
	require.NoError(t, intentos.Anadir(ctx, i))

	leido, err := NuevosIntentos(almacen).Intento(ctx, "i1")
	require.NoError(t, err)
	require.Equal(t, i.Registros(), leido.Registros())
	require.Equal(t, secreto, leido.Valores("test")["DB_PASSWORD"])

	err = filepath.WalkDir(raiz, func(ruta string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		datos, err := os.ReadFile(ruta)
		require.NoError(t, err)
		require.NotContains(t, string(datos), secreto, "el valor no queda en claro en %s", ruta)
		return nil
	})
	require.NoError(t, err)

	todos, err := intentos.Recorrer(ctx)
	require.NoError(t, err)
	require.Len(t, todos, 2)
}

// Una máquina que siguió viva después de que dieran su intento por abandonado lo tiene leído de antes, y no
// puede escribir detrás del abandono (IT-07 DEC-07.8).
func TestIntentosEnAlmacen_UnaCopiaLeidaAntesNoEscribeDetrasDeOtra(t *testing.T) {
	ctx := contexto()
	intentos := NuevosIntentos(NuevoAlmacenEnMemoria())

	i := dominio.NuevoIntento("i1")
	require.NoError(t, i.Abrir(aperturaDePrueba(), en(0), dominio.Contenido{}))
	require.NoError(t, intentos.Anadir(ctx, i))

	enLaMaquina, err := intentos.Intento(ctx, "i1")
	require.NoError(t, err)
	desdeElPortal, err := intentos.Intento(ctx, "i1")
	require.NoError(t, err)

	require.NoError(t, desdeElPortal.Abandonar(en(1)))
	require.NoError(t, intentos.Anadir(ctx, desdeElPortal))

	require.NoError(t, enLaMaquina.Comenzar("test", en(2), dominio.Contenido{}), "en su memoria sigue abierto")
	require.ErrorIs(t, intentos.Anadir(ctx, enLaMaquina), dominio.ErrConflicto)

	releido, err := intentos.Intento(ctx, "i1")
	require.NoError(t, err)
	require.True(t, releido.Abandonado())
	require.ErrorIs(t, releido.Comenzar("test", en(3), dominio.Contenido{}), dominio.ErrRechazado)
}

func TestRepositorios_DesplieguesOcupacionesLanzamientosYReservas(t *testing.T) {
	ctx := contexto()
	almacen, err := NuevoAlmacenLocal(t.TempDir())
	require.NoError(t, err)

	ocupaciones := NuevasOcupaciones(almacen)
	ocupacion, err := ocupaciones.DeUnAmbiente(ctx, "staging")
	require.NoError(t, err)
	require.NoError(t, ocupacion.Ocupar("i1", en(0), nil))
	require.NoError(t, ocupaciones.Anadir(ctx, ocupacion))
	releida, err := ocupaciones.DeUnAmbiente(ctx, "staging")
	require.NoError(t, err)
	require.Equal(t, []dominio.IdIntento{"i1"}, releida.Intentos())

	i := dominio.NuevoIntento("i1")
	require.NoError(t, i.Abrir(aperturaDePrueba(), en(0), dominio.Contenido{}))
	for _, paso := range []dominio.NombrePaso{"test", "supply", "deploy"} {
		require.NoError(t, i.Comenzar(paso, en(1), dominio.Contenido{}))
		require.NoError(t, i.Terminar(paso, true, en(1), dominio.Contenido{}))
	}
	despliegues := NuevosDespliegues(almacen)
	deStaging, err := despliegues.DeUnAmbiente(ctx, "staging")
	require.NoError(t, err)
	require.NoError(t, i.Cerrar(dominio.Cierre{Estado: dominio.Exitoso}, en(2), deStaging))
	d, _, err := i.Desplegar(deStaging, "d1", en(2))
	require.NoError(t, err)
	require.NoError(t, despliegues.Anadir(ctx, deStaging))
	releidos, err := despliegues.DeUnAmbiente(ctx, "staging")
	require.NoError(t, err)
	require.Equal(t, []dominio.Despliegue{d}, releidos.Todos())
	todos, err := despliegues.Recorrer(ctx)
	require.NoError(t, err)
	require.Len(t, todos, 1)

	lanzamientos := NuevosLanzamientos(almacen)
	historial, err := lanzamientos.Todos(ctx)
	require.NoError(t, err)
	l, err := historial.Lanzar("l1", releidos, "d1", en(3),
		dominio.Contenido{Contexto: "lanzamiento", Datos: []byte(`{"version":1}`)})
	require.NoError(t, err)
	require.NoError(t, lanzamientos.Anadir(ctx, historial))
	releido, err := lanzamientos.Todos(ctx)
	require.NoError(t, err)
	require.Equal(t, []dominio.Lanzamiento{l}, releido.Todos())

	reservas := NuevasReservas(almacen)
	deProd, err := reservas.DeUnAmbiente(ctx, "prod")
	require.NoError(t, err)
	require.NoError(t, deProd.Reservar(true, en(4)))
	require.NoError(t, reservas.Anadir(ctx, deProd))
	releidas, err := reservas.DeUnAmbiente(ctx, "prod")
	require.NoError(t, err)
	ultima, ok := releidas.Ultima()
	require.True(t, ok)
	require.Equal(t, dominio.Reserva{Reservado: true, Instante: en(4)}, ultima)
}
