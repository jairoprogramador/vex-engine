package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
)

func TestNuevoLanzamiento_ConNombreLoConserva(t *testing.T) {
	l := dominio.NuevoLanzamiento(idDespliegue(t, "d1"), hash(t, "h1"), version(t, 3), "listo-para-el-cliente")

	require.Equal(t, dominio.Nombre("listo-para-el-cliente"), l.Nombre())
	require.Equal(t, idDespliegue(t, "d1"), l.Despliegue())
	require.Equal(t, hash(t, "h1"), l.HashDelCodigo())
	require.Equal(t, version(t, 3), l.Version())
}

func TestNuevoLanzamiento_SinNombreTomaLaVersion(t *testing.T) {
	l := dominio.NuevoLanzamiento(idDespliegue(t, "d1"), hash(t, "h1"), version(t, 3), "")

	require.Equal(t, dominio.Nombre("3"), l.Nombre())
}
