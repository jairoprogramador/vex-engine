package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestUnResultadoDeUnComandoGuardaLoQueProdujo(t *testing.T) {
	r := dominio.ResultadoDeUnComando{
		Exitoso:    true,
		Producidas: []dominio.VariableProducida{{Nombre: "tag", Valor: "v1", Compartida: true}},
	}
	require.True(t, r.Exitoso)
	require.Equal(t, "tag", r.Producidas[0].Nombre)
	require.Equal(t, "v1", r.Producidas[0].Valor)
	require.True(t, r.Producidas[0].Compartida)
}
