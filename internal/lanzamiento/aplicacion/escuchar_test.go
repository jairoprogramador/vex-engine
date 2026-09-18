package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlRegistrarseUnDespliegue_NoReservadoYNuncaLanzado_Lanza(t *testing.T) {
	h := nuevoHistorialFalso()
	h.hashes["d1"] = hash(t, "h1")
	s := nuevoServicio(h)

	err := s.AlRegistrarseUnDespliegue(context.Background(), "staging", "d1")

	require.NoError(t, err)
	require.Len(t, h.lanzamientos, 1)
	require.Equal(t, idDespliegue(t, "d1"), h.lanzamientos[0].lanzamiento.Despliegue())
}

func TestAlRegistrarseUnDespliegue_NoReservadoYYaLanzadoEsteDespliegue_NoHaceNada(t *testing.T) {
	h := nuevoHistorialFalso()
	h.hashes["d1"] = hash(t, "h1")
	h.ultimo["staging"] = idDespliegue(t, "d1")
	s := nuevoServicio(h)

	err := s.AlRegistrarseUnDespliegue(context.Background(), "staging", "d1")

	require.NoError(t, err)
	require.Empty(t, h.lanzamientos)
}

func TestAlRegistrarseUnDespliegue_Reservado_NoHaceNadaPaseLoQuePase(t *testing.T) {
	h := nuevoHistorialFalso()
	h.hashes["d1"] = hash(t, "h1")
	h.reservado["prod"] = true
	s := nuevoServicio(h)

	err := s.AlRegistrarseUnDespliegue(context.Background(), "prod", "d1")

	require.NoError(t, err)
	require.Empty(t, h.lanzamientos)
}

func TestAlRegistrarseUnDespliegue_NoReservadoYUltimoLanzamientoDeOtroDespliegue_Lanza(t *testing.T) {
	h := nuevoHistorialFalso()
	h.hashes["d2"] = hash(t, "h2")
	h.ultimo["staging"] = idDespliegue(t, "d1")
	s := nuevoServicio(h)

	err := s.AlRegistrarseUnDespliegue(context.Background(), "staging", "d2")

	require.NoError(t, err)
	require.Len(t, h.lanzamientos, 1)
}
