package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
)

func idDespliegue(t *testing.T, valor string) dominio.IdDespliegue {
	t.Helper()
	d, err := dominio.NuevoIdDespliegue(valor)
	require.NoError(t, err)
	return d
}

func hash(t *testing.T, valor string) dominio.HashDelCodigo {
	t.Helper()
	h, err := dominio.NuevoHashDelCodigo(valor)
	require.NoError(t, err)
	return h
}

func version(t *testing.T, numero int) dominio.Version {
	t.Helper()
	v, err := dominio.NuevaVersion(numero)
	require.NoError(t, err)
	return v
}
