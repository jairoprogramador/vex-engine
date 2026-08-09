package deployment_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
)

const hashDePrueba = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

func TestContentID(t *testing.T) {
	t.Run("la forma canónica lleva el prefijo de versión", func(t *testing.T) {
		id, err := deployment.ParseContentID(deployment.ContentIDVersion + ":" + hashDePrueba)
		require.NoError(t, err)

		assert.Equal(t, deployment.ContentIDVersion, id.Version())
		assert.Equal(t, hashDePrueba, id.Hash())
		assert.Equal(t, deployment.ContentIDVersion+":"+hashDePrueba, id.String())
		assert.False(t, id.IsZero())
	})

	t.Run("una forma que no valida es un error", func(t *testing.T) {
		casos := map[string]string{
			"sin separador":       "cnt-v1" + hashDePrueba,
			"sin versión":         ":" + hashDePrueba,
			"hash corto":          "cnt-v1:abc",
			"hash en mayúsculas":  "cnt-v1:" + strings.ToUpper(hashDePrueba),
			"hash no hexadecimal": "cnt-v1:" + strings.Repeat("z", 64),
		}
		for nombre, texto := range casos {
			t.Run(nombre, func(t *testing.T) {
				_, err := deployment.ParseContentID(texto)
				require.Error(t, err)
			})
		}
	})

	t.Run("dos identidades de versiones distintas NUNCA son iguales", func(t *testing.T) {
		unaV1, err := deployment.ParseContentID("cnt-v1:" + hashDePrueba)
		require.NoError(t, err)
		unaV2, err := deployment.ParseContentID("cnt-v2:" + hashDePrueba)
		require.NoError(t, err)

		assert.False(t, unaV1.Equals(unaV2), "el mismo hash bajo otra regla es otra identidad")
	})

	t.Run("el valor cero es explícitamente «sin identidad»", func(t *testing.T) {
		var id deployment.ContentID
		assert.True(t, id.IsZero())
		assert.Empty(t, id.String(), "la cadena vacía no debe confundirse con un hash")
	})
}

func TestDeploymentID(t *testing.T) {
	contenido := contentIDDePrueba(t, "aa")

	t.Run("un contenido sin padre abre el linaje", func(t *testing.T) {
		id, err := deployment.DeploymentIDOf(contenido, deployment.DeploymentID{})
		require.NoError(t, err)

		assert.Equal(t, deployment.DeploymentIDVersion, id.Version())
		assert.False(t, id.IsZero())
	})

	t.Run("el MISMO contenido dos veces cae en dos posiciones distintas", func(t *testing.T) {
		primero, err := deployment.DeploymentIDOf(contenido, deployment.DeploymentID{})
		require.NoError(t, err)
		segundo, err := deployment.DeploymentIDOf(contenido, primero)
		require.NoError(t, err)

		assert.False(t, primero.Equals(segundo),
			"deployment_id cambia en cada ejecución nueva al mismo ambiente: "+
				"por eso la decisión de saltar un step no puede consultarlo")
	})

	t.Run("es determinista: mismo par, misma posición", func(t *testing.T) {
		padre, err := deployment.DeploymentIDOf(contenido, deployment.DeploymentID{})
		require.NoError(t, err)

		unaVez, err := deployment.DeploymentIDOf(contenido, padre)
		require.NoError(t, err)
		otraVez, err := deployment.DeploymentIDOf(contenido, padre)
		require.NoError(t, err)

		assert.True(t, unaVez.Equals(otraVez))
	})

	t.Run("dos contenidos distintos bajo el mismo padre caen en posiciones distintas", func(t *testing.T) {
		padre, err := deployment.DeploymentIDOf(contenido, deployment.DeploymentID{})
		require.NoError(t, err)

		uno, err := deployment.DeploymentIDOf(contenido, padre)
		require.NoError(t, err)
		otro, err := deployment.DeploymentIDOf(contentIDDePrueba(t, "bb"), padre)
		require.NoError(t, err)

		assert.False(t, uno.Equals(otro))
	})

	t.Run("compone sobre la forma canónica COMPLETA, no sobre el hash pelado", func(t *testing.T) {
		mismoHash := strings.Repeat("aa", 32)
		v1, err := deployment.ParseContentID("cnt-v1:" + mismoHash)
		require.NoError(t, err)
		v2, err := deployment.ParseContentID("cnt-v2:" + mismoHash)
		require.NoError(t, err)

		desdeV1, err := deployment.DeploymentIDOf(v1, deployment.DeploymentID{})
		require.NoError(t, err)
		desdeV2, err := deployment.DeploymentIDOf(v2, deployment.DeploymentID{})
		require.NoError(t, err)

		assert.False(t, desdeV1.Equals(desdeV2),
			"un salto de versión del contenido tiene que invalidar lo derivado")
	})

	t.Run("un contenido sin identidad no deriva una posición degradada", func(t *testing.T) {
		_, err := deployment.DeploymentIDOf(deployment.ContentID{}, deployment.DeploymentID{})
		require.Error(t, err)
	})

	t.Run("la forma canónica se lee y se escribe igual", func(t *testing.T) {
		id, err := deployment.DeploymentIDOf(contenido, deployment.DeploymentID{})
		require.NoError(t, err)

		leido, err := deployment.ParseDeploymentID(id.String())
		require.NoError(t, err)
		assert.True(t, id.Equals(leido))

		_, err = deployment.ParseDeploymentID("dep-v1:corto")
		require.Error(t, err)
	})

	t.Run("el valor cero es explícitamente «sin identidad»", func(t *testing.T) {
		var id deployment.DeploymentID
		assert.True(t, id.IsZero())
		assert.Empty(t, id.String())
	})
}

func contentIDDePrueba(t *testing.T, par string) deployment.ContentID {
	t.Helper()

	id, err := deployment.ParseContentID(
		deployment.ContentIDVersion + ":" + strings.Repeat(par, 32))
	require.NoError(t, err)
	return id
}
