package dominio_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/suministro/dominio"
)

func hash(t *testing.T) dominio.Hash {
	t.Helper()
	h, err := dominio.NuevoHash("contenido-v1:abc")
	require.NoError(t, err)
	return h
}

func commit(t *testing.T) dominio.Commit {
	t.Helper()
	c, err := dominio.NuevoCommit(strings.Repeat("a1", 20))
	require.NoError(t, err)
	return c
}

func TestUnCommitEsSuIdentificadorCompleto(t *testing.T) {
	sha1 := strings.Repeat("a1", 20)
	casos := []struct {
		nombre string
		id     string
		valido bool
	}{
		{"sha-1", sha1, true},
		{"sha-256", strings.Repeat("b2", 32), true},
		{"en mayúsculas es el mismo", strings.ToUpper(sha1), true},
		{"vacío", "", false},
		{"abreviado", sha1[:7], false},
		{"una rama no es un punto fijo", "main", false},
		{"no hexadecimal", strings.Repeat("zz", 20), false},
		{"con un espacio en medio", sha1[:20] + " " + sha1[21:], false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			commit, err := dominio.NuevoCommit(c.id)
			if !c.valido {
				require.ErrorIs(t, err, dominio.ErrInvalido)
				return
			}
			require.NoError(t, err)
			require.Equal(t, strings.ToLower(c.id), commit.String())
		})
	}
}

func TestLoQueSePideDiceDondeEsta(t *testing.T) {
	_, err := dominio.NuevaFuente("")
	require.ErrorIs(t, err, dominio.ErrInvalido)

	_, err = dominio.NuevaCopiaDeTrabajo("")
	require.ErrorIs(t, err, dominio.ErrInvalido)

	_, err = dominio.NuevoHash("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnaCopiaDeTrabajoTieneHashYNoTieneCommit(t *testing.T) {
	material, err := dominio.MaterialDeUnaCopiaDeTrabajo("/material", hash(t))
	require.NoError(t, err)

	_, conCommit := material.Commit()
	require.False(t, conCommit)
	require.Equal(t, hash(t), material.Hash())
	require.Equal(t, "/material", material.Directorio())
}

func TestElMaterialDeUnCommitDiceSuCommit(t *testing.T) {
	_, err := dominio.MaterialDeUnCommit("/material", hash(t), dominio.Commit{})
	require.Error(t, err)

	material, err := dominio.MaterialDeUnCommit("/material", hash(t), commit(t))
	require.NoError(t, err)
	suyo, conCommit := material.Commit()
	require.True(t, conCommit)
	require.Equal(t, commit(t), suyo)
}

func TestUnMaterialTieneDirectorioYHash(t *testing.T) {
	_, err := dominio.MaterialDeUnaCopiaDeTrabajo("", hash(t))
	require.Error(t, err)

	_, err = dominio.MaterialDeUnCommit("/material", dominio.Hash{}, commit(t))
	require.Error(t, err)
}
