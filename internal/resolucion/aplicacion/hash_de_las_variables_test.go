package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

// Lo que cuenta como «las variables de un paso»: las del pipeline de su ámbito, las que produjeron pasos
// anteriores y los metadatos. No las generadas por el motor.

func definicionDePrueba(t *testing.T, declaradas ...dominio.VariableDeclarada) *definicionFalsa {
	t.Helper()
	return &definicionFalsa{
		estandar: []dominio.VariableEstandar{
			{Nombre: "project_name", Metadato: true},
			{Nombre: "project_workdir"}, // generada: cambia en cada intento
		},
		declaradas: declaradas,
	}
}

// hashDeUnIntento declara las variables del paso en un intento nuevo y devuelve su hash.
func hashDeUnIntento(t *testing.T, s *aplicacion.Servicio, intento string, estandar map[string]string) string {
	t.Helper()
	ctx := context.Background()
	_, err := s.ParaEjecucion().VariablesDeUnPaso(ctx, intento, "paso", ambitoProdPublicado(), "fuente", "commit", estandar)
	require.NoError(t, err)
	hash, err := s.ParaEjecucion().HashDeLasVariables(ctx, intento, ambitoProdPublicado())
	require.NoError(t, err)
	return hash
}

func TestHashDeLasVariables_UnaGeneradaQueCambiaNoCambiaElHash(t *testing.T) {
	s := nuevoServicio(t, nil, definicionDePrueba(t))

	primero := hashDeUnIntento(t, s, "i1", map[string]string{"project_name": "vex", "project_workdir": "/tmp/material-1"})
	segundo := hashDeUnIntento(t, s, "i2", map[string]string{"project_name": "vex", "project_workdir": "/tmp/material-2"})

	require.Equal(t, primero, segundo)
}

func TestHashDeLasVariables_UnMetadatoQueCambiaCambiaElHash(t *testing.T) {
	s := nuevoServicio(t, nil, definicionDePrueba(t))

	primero := hashDeUnIntento(t, s, "i1", map[string]string{"project_name": "vex", "project_workdir": "/tmp/a"})
	segundo := hashDeUnIntento(t, s, "i2", map[string]string{"project_name": "otro", "project_workdir": "/tmp/a"})

	require.NotEqual(t, primero, segundo)
}

func TestHashDeLasVariables_UnLiteralDelPipelineQueCambiaCambiaElHash(t *testing.T) {
	prod := mustAmbito(t, "prod")
	antes := nuevoServicio(t, nil, definicionDePrueba(t, dominio.VariableDeclarada{Nombre: "replicas", Ambito: prod, Valor: "2"}))
	despues := nuevoServicio(t, nil, definicionDePrueba(t, dominio.VariableDeclarada{Nombre: "replicas", Ambito: prod, Valor: "5"}))
	estandar := map[string]string{"project_name": "vex"}

	require.NotEqual(t, hashDeUnIntento(t, antes, "i1", estandar), hashDeUnIntento(t, despues, "i1", estandar))
}

func TestHashDeLasVariables_UnLiteralDeOtroAmbitoNoEntra(t *testing.T) {
	sin := nuevoServicio(t, nil, definicionDePrueba(t))
	conStag := nuevoServicio(t, nil, definicionDePrueba(t,
		dominio.VariableDeclarada{Nombre: "replicas", Ambito: mustAmbito(t, "stag"), Valor: "9"}))
	estandar := map[string]string{"project_name": "vex"}

	require.Equal(t, hashDeUnIntento(t, sin, "i1", estandar), hashDeUnIntento(t, conStag, "i1", estandar),
		"desde prod no se ve lo que solo existe en stag")
}

func TestHashDeLasVariables_LoQueProdujoUnPasoAnteriorEntraYSuCambioSeNota(t *testing.T) {
	ctx := context.Background()
	s := nuevoServicio(t, nil, definicionDePrueba(t))
	estandar := map[string]string{"project_name": "vex"}

	hash := func(intento, etiqueta string) string {
		_, err := s.ParaEjecucion().VariablesDeUnPaso(ctx, intento, "paso", ambitoProdPublicado(), "fuente", "commit", estandar)
		require.NoError(t, err)
		require.NoError(t, s.ParaEjecucion().RegistrarProducido(ctx, intento, "anterior", "etiqueta", etiqueta, ambitoProdPublicado()))
		h, err := s.ParaEjecucion().HashDeLasVariables(ctx, intento, ambitoProdPublicado())
		require.NoError(t, err)
		return h
	}

	require.Equal(t, hash("i1", "v1"), hash("i2", "v1"))
	require.NotEqual(t, hash("i3", "v1"), hash("i4", "v2"))
}
