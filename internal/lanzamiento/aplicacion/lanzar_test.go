package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
)

func TestLanzar_ConNombreLoConserva(t *testing.T) {
	h := nuevoHistorialFalso()
	h.hashes["d1"] = hash(t, "h1")
	s := nuevoServicio(h)

	l, err := s.Lanzar(context.Background(), "staging", "d1", "listo-para-el-cliente")

	require.NoError(t, err)
	require.Equal(t, "listo-para-el-cliente", l.Nombre)
	require.Equal(t, 1, l.Version)
	require.Equal(t, "staging", l.Ambiente)
	require.Equal(t, "d1", l.Despliegue)
}

func TestLanzar_DevuelveLaIdentidadQueDecidioElHistorial(t *testing.T) {
	h := nuevoHistorialFalso()
	h.hashes["d1"] = hash(t, "h1")
	s := nuevoServicio(h)

	l, err := s.Lanzar(context.Background(), "staging", "d1", "")

	require.NoError(t, err)
	require.Equal(t, "lz-1", l.Id)
}

func TestLanzar_SinNombreTomaLaVersion(t *testing.T) {
	h := nuevoHistorialFalso()
	h.hashes["d1"] = hash(t, "h1")
	s := nuevoServicio(h)

	l, err := s.Lanzar(context.Background(), "staging", "d1", "")

	require.NoError(t, err)
	require.Equal(t, "1", l.Nombre)
}

func TestLanzar_EsIncondicional_LanzaAunqueElAmbienteEsteReservado(t *testing.T) {
	h := nuevoHistorialFalso()
	h.hashes["d1"] = hash(t, "h1")
	h.reservado["prod"] = true
	s := nuevoServicio(h)

	_, err := s.Lanzar(context.Background(), "prod", "d1", "")

	require.NoError(t, err)
	require.Len(t, h.lanzamientos, 1)
}

func TestLanzar_ElMismoHashEnDosAmbientesTomaLaMismaVersion(t *testing.T) {
	h := nuevoHistorialFalso()
	h.hashes["d1"] = hash(t, "h1")
	h.hashes["d2"] = hash(t, "h1")
	s := nuevoServicio(h)

	l1, err := s.Lanzar(context.Background(), "staging", "d1", "")
	require.NoError(t, err)
	h.conocidas = []dominio.VersionConocida{{Hash: hash(t, "h1"), Version: version(t, l1.Version)}}

	l2, err := s.Lanzar(context.Background(), "prod", "d2", "")
	require.NoError(t, err)

	require.Equal(t, l1.Version, l2.Version)
}

func TestLanzar_UnAmbienteVacioEsInvalido(t *testing.T) {
	s := nuevoServicio(nuevoHistorialFalso())

	_, err := s.Lanzar(context.Background(), "", "d1", "")

	require.ErrorIs(t, err, publicado.ErrInvalido)
}

func TestLanzar_UnDespliegueVacioEsInvalido(t *testing.T) {
	s := nuevoServicio(nuevoHistorialFalso())

	_, err := s.Lanzar(context.Background(), "staging", "", "")

	require.ErrorIs(t, err, publicado.ErrInvalido)
}

func TestLanzar_PropagaElErrorDeRegistrarLanzamiento(t *testing.T) {
	h := nuevoHistorialFalso()
	h.hashes["d1"] = hash(t, "h1")
	h.fallarRegistrarLanzamiento = true
	s := nuevoServicio(h)

	_, err := s.Lanzar(context.Background(), "staging", "d1", "")

	require.Error(t, err)
	require.NotErrorIs(t, err, publicado.ErrInvalido, "no es un error de validación, es del Historial")
}
