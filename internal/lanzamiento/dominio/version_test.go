package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
)

func TestDecidirVersion_UnHashNuevoSinConocidasEmpiezaEnUno(t *testing.T) {
	v := dominio.DecidirVersion(hash(t, "h1"), nil)

	require.Equal(t, 1, v.Numero())
}

func TestDecidirVersion_UnHashNuevoTomaLaSiguienteALaMayorConocida(t *testing.T) {
	conocidas := []dominio.VersionConocida{
		{Hash: hash(t, "h1"), Version: version(t, 1)},
		{Hash: hash(t, "h2"), Version: version(t, 3)},
		{Hash: hash(t, "h3"), Version: version(t, 2)},
	}

	v := dominio.DecidirVersion(hash(t, "h4"), conocidas)

	require.Equal(t, 4, v.Numero())
}

func TestDecidirVersion_UnHashYaConocidoReusaSuVersionSinImportarSuPosicion(t *testing.T) {
	conocidas := []dominio.VersionConocida{
		{Hash: hash(t, "h1"), Version: version(t, 1)},
		{Hash: hash(t, "h2"), Version: version(t, 5)},
		{Hash: hash(t, "h3"), Version: version(t, 2)},
	}

	v := dominio.DecidirVersion(hash(t, "h2"), conocidas)

	require.Equal(t, 5, v.Numero())
}
