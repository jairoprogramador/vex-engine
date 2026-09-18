package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func destinoValido(t *testing.T) dominio.Destino {
	t.Helper()
	d, err := dominio.NuevoDestino("dep-1", "prod", "git@proyecto", "c1", "git@pipeline", "c2")
	require.NoError(t, err)
	return d
}

func TestUnDestinoNoPuedeTenerElDesplieguevacio(t *testing.T) {
	_, err := dominio.NuevoDestino("", "prod", "fp", "cp", "fpl", "cpl")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnDestinoNecesitaLaFuenteYElCommitDelProyecto(t *testing.T) {
	_, err := dominio.NuevoDestino("dep-1", "prod", "", "c1", "fpl", "cpl")
	require.ErrorIs(t, err, dominio.ErrInvalido)

	_, err = dominio.NuevoDestino("dep-1", "prod", "fp", "", "fpl", "cpl")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnDestinoNecesitaLaFuenteYElCommitDelPipeline(t *testing.T) {
	_, err := dominio.NuevoDestino("dep-1", "prod", "fp", "cp", "", "cpl")
	require.ErrorIs(t, err, dominio.ErrInvalido)

	_, err = dominio.NuevoDestino("dep-1", "prod", "fp", "cp", "fpl", "")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnDestinoGuardaLosDosCommits(t *testing.T) {
	d := destinoValido(t)
	require.Equal(t, "dep-1", d.Despliegue())
	require.Equal(t, "prod", d.Ambiente())
	require.Equal(t, "git@proyecto", d.FuenteDelProyecto())
	require.Equal(t, "c1", d.CommitDelProyecto())
	require.Equal(t, "git@pipeline", d.FuenteDelPipeline())
	require.Equal(t, "c2", d.CommitDelPipeline())
}
