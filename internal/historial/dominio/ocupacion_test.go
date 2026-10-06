package dominio_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

func TestOcupacion_UnAmbienteTieneComoMuchoUnIntentoEnCurso(t *testing.T) {
	o := dominio.NuevaOcupacion("staging")
	require.NoError(t, o.Ocupar("i1", en(0), nil), "un ambiente libre")

	enCurso := abierto(t, "i1", aperturaEn("staging"))
	err := o.Ocupar("i2", en(1), enCurso)
	var ocupado *dominio.AmbienteOcupadoError
	require.ErrorAs(t, err, &ocupado)
	require.Equal(t, dominio.IdIntento("i1"), ocupado.Intento, "dice cuál es el intento en curso")
	require.ErrorIs(t, err, dominio.ErrRechazado)

	require.ErrorIs(t, o.Ocupar("i2", en(1), nil), dominio.ErrRechazado, "sin el intento de la última ocupación")
	require.ErrorIs(t, o.Ocupar("i2", en(1), abierto(t, "otro", aperturaEn("staging"))), dominio.ErrRechazado,
		"con otro intento que el de la última ocupación")
	require.ErrorAs(t, o.Ocupar("i2", en(1), dominio.NuevoIntento("i1")), &ocupado,
		"un intento sin apertura también ocupa: puede estar escribiéndola en otra máquina")

	cerrar(t, enCurso, dominio.Fallido)
	require.NoError(t, o.Ocupar("i2", en(2), enCurso), "el cierre libera")

	abandonado := abierto(t, "i2", aperturaEn("staging"))
	require.NoError(t, abandonado.Abandonar(en(3)))
	require.NoError(t, o.Ocupar("i3", en(4), abandonado), "el abandono libera")

	require.Equal(t, []dominio.IdIntento{"i1", "i2", "i3"}, o.Intentos())
	require.Len(t, o.Nuevos(), 3)
}

func TestReconstituirOcupacion_UnIntentoOcupaUnaSolaVez(t *testing.T) {
	_, err := dominio.ReconstituirOcupacion("staging", []dominio.RegistroDeOcupacion{
		{Intento: "i1", Instante: en(0)}, {Intento: "i1", Instante: en(1)},
	})
	require.ErrorIs(t, err, dominio.ErrRechazado)
}

func TestLanzar_SuDespliegueExisteYEsDeEseAmbiente(t *testing.T) {
	despliegues, err := dominio.ReconstituirDesplieguesDeUnAmbiente("staging", []dominio.Despliegue{
		dominio.ReconstituirDespliegue("d1", "staging", "i1", "", en(1)),
	})
	require.NoError(t, err)
	lanzamientos := dominio.ReconstituirLanzamientos(nil)

	_, err = lanzamientos.Lanzar("l1", despliegues, "d9", en(2), nada)
	require.ErrorIs(t, err, dominio.ErrRechazado, "un despliegue que no es de ese ambiente")
	_, err = lanzamientos.Lanzar("l1", nil, "d1", en(2), nada)
	require.ErrorIs(t, err, dominio.ErrRechazado)

	version := dominio.Contenido{Contexto: "lanzamiento", Datos: []byte(`{"version":1}`)}
	l, err := lanzamientos.Lanzar("l1", despliegues, "d1", en(2), version)
	require.NoError(t, err)
	require.Equal(t, dominio.Ambiente("staging"), l.Ambiente())

	ultimo, ok := lanzamientos.UltimoDeUnAmbiente("staging")
	require.True(t, ok)
	require.Equal(t, l, ultimo)
	_, ok = lanzamientos.UltimoDeUnAmbiente("prod")
	require.False(t, ok)
}

func TestReservas_LaUltimaDiceSiElAmbienteEstaReservado(t *testing.T) {
	reservas := dominio.ReconstituirReservas("prod", nil)
	_, ok := reservas.Ultima()
	require.False(t, ok)

	require.NoError(t, reservas.Reservar(true, en(1)))
	require.NoError(t, reservas.Reservar(false, en(2)))
	ultima, ok := reservas.Ultima()
	require.True(t, ok)
	require.False(t, ultima.Reservado)
	require.ErrorIs(t, reservas.Reservar(true, time.Time{}), dominio.ErrRechazado)
}

// IT-07 DEC-07.5: un registro es un hecho que no cambia, y ningún repositorio puede cambiarlo.
func TestRepositorios_SoloAnadenYRecorren(t *testing.T) {
	puertos := []reflect.Type{
		reflect.TypeFor[dominio.Intentos](),
		reflect.TypeFor[dominio.Despliegues](),
		reflect.TypeFor[dominio.Ocupaciones](),
		reflect.TypeFor[dominio.Lanzamientos](),
		reflect.TypeFor[dominio.Reservas](),
	}
	prohibidos := []string{"Actualizar", "Borrar", "Eliminar", "Modificar", "Reemplazar", "Sustituir", "Quitar", "Guardar"}
	for _, puerto := range puertos {
		escrituras := 0
		for k := 0; k < puerto.NumMethod(); k++ {
			nombre := puerto.Method(k).Name
			for _, prohibido := range prohibidos {
				require.False(t, strings.HasPrefix(nombre, prohibido), "%s.%s", puerto.Name(), nombre)
			}
			if nombre == "Anadir" {
				escrituras++
			}
		}
		require.Equal(t, 1, escrituras, "%s solo escribe con Anadir", puerto.Name())
	}
}
