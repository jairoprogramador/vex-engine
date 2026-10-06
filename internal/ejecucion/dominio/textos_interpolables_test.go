package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestTextosInterpolablesSonLasLineasDeLosComandosYElContenidoDeLasPlantillas(t *testing.T) {
	comandos := []dominio.ComandoDeclarado{
		comandoDePrueba(t, "echo ${var.a}", nil),
		comandoDePrueba(t, "echo ${var.b}", nil),
	}
	material := []dominio.FicheroDeclarado{
		ficheroDePrueba(t, "plantilla.tf", "region=${var.c}", true),
		ficheroDePrueba(t, "fijo.txt", "${var.d}", false),
	}

	require.Equal(t,
		[]string{"echo ${var.a}", "echo ${var.b}", "region=${var.c}"},
		dominio.TextosInterpolables(comandos, material))
}
