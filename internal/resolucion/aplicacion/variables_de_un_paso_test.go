package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

func TestVariablesDeUnPaso_DeclaraLasDelPipelineYLasEstandar(t *testing.T) {
	def := &definicionFalsa{
		estandar:   []dominio.VariableEstandar{{Nombre: "ejecucion_id", Metadato: true, DelPaso: false}},
		declaradas: []dominio.VariableDeclarada{{Nombre: "host", Ambito: mustAmbito(t, "prod"), Valor: "vexja.com"}},
	}
	s := nuevoServicio(t, nil, def)
	ctx := context.Background()

	variables, err := s.ParaEjecucion().VariablesDeUnPaso(
		ctx, "intento-1", "paso", ambitoProdPublicado(), "fuente", "commit", map[string]string{"ejecucion_id": "abc123"})
	require.NoError(t, err)

	var nombres []string
	for _, v := range variables {
		nombres = append(nombres, v.Nombre)
	}
	require.ElementsMatch(t, []string{"ejecucion_id", "host"}, nombres)
}

func TestVariablesDeUnPaso_InterpolaUnLiteralQueUsaOtraVariable(t *testing.T) {
	prod := mustAmbito(t, "prod")
	def := &definicionFalsa{
		declaradas: []dominio.VariableDeclarada{
			// deliberadamente en el orden que NO respeta la dependencia (RD-04 §9.16: el orden no es de
			// dependencia): "url" antes que "host", al que usa.
			{Nombre: "url", Ambito: prod, Valor: "https://${var.host}/"},
			{Nombre: "host", Ambito: prod, Valor: "vexja.com"},
		},
	}
	s := nuevoServicio(t, nil, def)
	ctx := context.Background()

	variables, err := s.ParaEjecucion().VariablesDeUnPaso(
		ctx, "intento-1", "paso", ambitoProdPublicado(), "fuente", "commit", nil)
	require.NoError(t, err)
	require.Len(t, variables, 2)

	texto, err := s.ParaEjecucion().Interpolar(ctx, "intento-1", "paso", ambitoProdPublicado(), "${var.url}")
	require.NoError(t, err)
	require.Equal(t, "https://vexja.com/", texto)
}

func TestVariablesDeUnPaso_UnPasoCompartidoNoRecibeDeclaradasDeUnAmbiente(t *testing.T) {
	prod := mustAmbito(t, "prod")
	def := &definicionFalsa{
		declaradas: []dominio.VariableDeclarada{
			{Nombre: "de-prod", Ambito: prod, Valor: "v"},
			{Nombre: "compartida", Ambito: dominio.AmbitoCompartido(), Valor: "v"},
		},
	}
	s := nuevoServicio(t, nil, def)
	ctx := context.Background()

	variables, err := s.ParaEjecucion().VariablesDeUnPaso(
		ctx, "intento-1", "paso-compartido", publicado.Ambito{Compartido: true}, "fuente", "commit", nil)
	require.NoError(t, err)

	var nombres []string
	for _, v := range variables {
		nombres = append(nombres, v.Nombre)
	}
	require.Equal(t, []string{"compartida"}, nombres)
}
