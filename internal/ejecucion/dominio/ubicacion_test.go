package dominio_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestNombreDeDirectorio_EsDeterministaYSeguro(t *testing.T) {
	a, err := dominio.NombreDeDirectorio("https://github.com/org/repo.git")
	require.NoError(t, err)
	b, err := dominio.NombreDeDirectorio("https://github.com/org/repo.git")
	require.NoError(t, err)
	require.Equal(t, a, b)
	require.Regexp(t, regexp.MustCompile(`^[0-9a-f]{16}$`), a)
}

func TestNombreDeDirectorio_VariantesDeLaMismaURLDanElMismoNombre(t *testing.T) {
	esperado, err := dominio.NombreDeDirectorio("github.com/org/repo")
	require.NoError(t, err)
	for _, v := range []string{
		"https://github.com/org/repo.git",
		"https://GITHUB.com/org/repo/",
		"https://usuario@github.com/org/repo",
		"ssh://git@github.com/org/repo.git",
		"git@github.com:org/repo.git",
		"  https://github.com/org/repo  ",
	} {
		t.Run(v, func(t *testing.T) {
			got, err := dominio.NombreDeDirectorio(v)
			require.NoError(t, err)
			require.Equal(t, esperado, got)
		})
	}
}

func TestNombreDeDirectorio_FuentesDistintasDanNombresDistintos(t *testing.T) {
	a, _ := dominio.NombreDeDirectorio("https://github.com/org/repo")
	b, _ := dominio.NombreDeDirectorio("https://github.com/org/Repo")
	c, _ := dominio.NombreDeDirectorio("repo")
	require.NotEqual(t, a, b, "la ruta distingue mayúsculas")
	require.NotEqual(t, a, c)
}

func TestNombreDeDirectorio_UnaReferenciaVaciaEsUnError(t *testing.T) {
	for _, v := range []string{"", "   "} {
		_, err := dominio.NombreDeDirectorio(v)
		require.ErrorIs(t, err, dominio.ErrInvalido)
	}
}
