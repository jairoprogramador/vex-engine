package syncconfig_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/syncconfig"
)

func TestNew_VocabularioCerrado(t *testing.T) {
	casos := []struct {
		nombre    string
		tipo      string
		path      string
		enElError string
	}{
		{
			nombre:    "sin tipo",
			tipo:      "",
			enElError: "no declara 'type'",
		},
		{
			nombre:    "un tipo inventado",
			tipo:      "supabase",
			enElError: `"supabase" desconocido`,
		},
		{
			nombre:    "local sin path",
			tipo:      "local",
			path:      "",
			enElError: "no declara 'local.path'",
		},
		{
			nombre:    "local con una ruta relativa",
			tipo:      "local",
			path:      "estado",
			enElError: "tiene que ser absoluta",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			_, err := syncconfig.New(caso.tipo, caso.path, "")

			require.Error(t, err)
			assert.Contains(t, err.Error(), caso.enElError)
			assert.NotErrorIs(t, err, syncconfig.ErrCongelado,
				"un error de configuración no puede leerse como «todavía no implementado»")
		})
	}
}

// §5.5: `http` está en el vocabulario y NO está implementado, y las dos cosas
// tienen que poder distinguirse desde fuera. Un `type: http` no es una errata.
func TestNew_HttpEstaCongeladoYSeDistingue(t *testing.T) {
	_, err := syncconfig.New("http", "", "https://ingesta.vex.test")

	require.Error(t, err)
	assert.ErrorIs(t, err, syncconfig.ErrCongelado)
	assert.Contains(t, err.Error(), "spec 26", "el error dice quién lo descongela")
}

func TestNew_LocalSeConstruyeYNormalizaLaRuta(t *testing.T) {
	raiz := filepath.Join(string(filepath.Separator), "mnt", "vex-state")

	cfg, err := syncconfig.New("  local  ", filepath.Join(raiz, "sub", ".."), "")

	require.NoError(t, err)
	assert.Equal(t, syncconfig.TypeLocal, cfg.Type())
	assert.Equal(t, raiz, cfg.Path())
	assert.False(t, cfg.IsZero())
}

// El valor cero no es «local a secas»: sin configuración no hay destino, que es
// la mitad de «el motor no tiene un default propio».
func TestConfig_ElValorCeroNoEsUnDestino(t *testing.T) {
	var cfg syncconfig.Config

	assert.True(t, cfg.IsZero())
	assert.Empty(t, string(cfg.Type()))
	assert.Empty(t, cfg.Path())
}
