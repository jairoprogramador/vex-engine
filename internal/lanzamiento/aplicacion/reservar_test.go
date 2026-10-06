package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReservar_RegistraLaReservaConVerdadero(t *testing.T) {
	h := nuevoHistorialFalso()
	s := nuevoServicio(h)

	require.NoError(t, s.Reservar(context.Background(), "prod"))

	require.Equal(t, []registroDeReserva{{ambiente: mustAmbiente(t, "prod"), reservado: true}}, h.reservas)
}

func TestLiberar_RegistraLaReservaConFalso(t *testing.T) {
	h := nuevoHistorialFalso()
	s := nuevoServicio(h)

	require.NoError(t, s.Liberar(context.Background(), "prod"))

	require.Equal(t, []registroDeReserva{{ambiente: mustAmbiente(t, "prod"), reservado: false}}, h.reservas)
}

func TestReservar_UnAmbienteVacioEsInvalido(t *testing.T) {
	s := nuevoServicio(nuevoHistorialFalso())

	require.Error(t, s.Reservar(context.Background(), ""))
}
