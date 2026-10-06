package aplicacion

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// Pruebas de caja blanca sobre funciones puras, sin puertos de por medio: el flujo completo (aplicacion.
// Intentar) se ejercita en intentar_test.go y en la prueba de punta a punta.

func TestEstandarCompartidasTraeLasCincoDeMetadatosYLasCuatroDelMotor(t *testing.T) {
	meta := publicado.Metadatos{
		ProjectId: "p1", ProjectName: "mi-proyecto", ProjectOrganization: "acme", ProjectTeam: "plataforma",
	}

	estandar := estandarCompartidas(meta, "prod", "contenido-v1:abc", "c-proyecto", "/ws/prod/motor", "vexd")

	require.Equal(t, map[string]string{
		"project_id":           "p1",
		"project_name":         "mi-proyecto",
		"project_organization": "acme",
		"project_team":         "plataforma",
		"environment":          "prod",
		"project_hash":         "contenido-v1:abc",
		"project_version":      "c-proyecto",
		"project_workdir":      "/ws/prod/motor",
		"tool_name":            "vexd",
	}, estandar)
}

func TestConElPasoAnadeStepNameYStepWorkdirSinMutarElOriginal(t *testing.T) {
	compartidas := estandarCompartidas(publicado.Metadatos{}, "prod", "h", "c", "/ws", "vexd")

	conPaso := conElPaso(compartidas, "01-pruebas", "/ws/prod/motor/01-pruebas")

	require.Equal(t, "01-pruebas", conPaso["step_name"])
	require.Equal(t, "/ws/prod/motor/01-pruebas", conPaso["step_workdir"])
	require.NotContains(t, compartidas, "step_name", "estandarCompartidas no debería mutarse por conElPaso")
}

func TestConElPasoCambiaEntrePasosSinAfectarseEntreSi(t *testing.T) {
	compartidas := estandarCompartidas(publicado.Metadatos{}, "prod", "h", "c", "/ws", "vexd")

	primero := conElPaso(compartidas, "01-pruebas", "/ws/prod/motor/01-pruebas")
	segundo := conElPaso(compartidas, "02-despliegue", "/ws/prod/motor/02-despliegue")

	require.Equal(t, "01-pruebas", primero["step_name"])
	require.Equal(t, "02-despliegue", segundo["step_name"])
}
