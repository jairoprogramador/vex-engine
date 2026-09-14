package deployment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
)

func TestLineage_SeAbreVacioYAvanza(t *testing.T) {
	contenido := materialBase(t).componer(t)

	linaje, err := deployment.LineageOf(contenido, deployment.DeploymentID{})
	require.NoError(t, err)
	assert.True(t, linaje.IsEmpty(), "un ambiente sin historia no tiene cabeza")
	assert.Equal(t, contenido.Subject(), linaje.Subject())
	assert.Equal(t, contenido.Destination(), linaje.Destination())

	tras, primero, err := linaje.Advance(contenido.ID())
	require.NoError(t, err)
	assert.False(t, tras.IsEmpty())
	assert.True(t, tras.Head().Equals(primero))

	// El linaje original no se movió: `Advance` devuelve uno nuevo.
	assert.True(t, linaje.IsEmpty())
}

func TestLineage_ElMismoContenidoDosVecesAvanzaLaHistoria(t *testing.T) {
	contenido := materialBase(t).componer(t)
	linaje, err := deployment.LineageOf(contenido, deployment.DeploymentID{})
	require.NoError(t, err)

	tras, primero, err := linaje.Advance(contenido.ID())
	require.NoError(t, err)
	_, segundo, err := tras.Advance(contenido.ID())
	require.NoError(t, err)

	assert.False(t, primero.Equals(segundo),
		"volver a desplegar lo mismo es otro despliegue, no el mismo")
}

func TestLineage_DosAmbientesSonDosHistorias(t *testing.T) {
	sand := materialBase(t).componer(t)
	prodMaterial := materialBase(t)
	prodMaterial.destination = "prod"
	prod := prodMaterial.componer(t)

	linajeSand, err := deployment.LineageOf(sand, deployment.DeploymentID{})
	require.NoError(t, err)
	linajeProd, err := deployment.LineageOf(prod, deployment.DeploymentID{})
	require.NoError(t, err)

	_, enSand, err := linajeSand.Advance(sand.ID())
	require.NoError(t, err)
	_, enProd, err := linajeProd.Advance(prod.ID())
	require.NoError(t, err)

	assert.False(t, enSand.Equals(enProd),
		"el destino ya viaja dentro del content_id, así que los linajes no colisionan")
	assert.False(t, linajeSand.Equals(linajeProd))
}

func TestLineage_UnLinajeSinAbrirNoAvanza(t *testing.T) {
	var linaje deployment.Lineage
	assert.True(t, linaje.IsZero())

	_, _, err := linaje.Advance(contentIDDePrueba(t, "aa"))
	require.Error(t, err)
}

func TestLineage_NoSeAbreSinSujetoNiSinDestino(t *testing.T) {
	sujeto, err := deployment.NewSubject(sujetoBase)
	require.NoError(t, err)
	destino, err := deployment.NewDestination(destinoBase)
	require.NoError(t, err)

	_, err = deployment.NewLineage(deployment.Subject{}, destino, deployment.DeploymentID{})
	require.Error(t, err)

	_, err = deployment.NewLineage(sujeto, deployment.Destination{}, deployment.DeploymentID{})
	require.Error(t, err)
}

func TestLineage_AvanzarConUnContenidoSinIdentidadEsUnError(t *testing.T) {
	contenido := materialBase(t).componer(t)
	linaje, err := deployment.LineageOf(contenido, deployment.DeploymentID{})
	require.NoError(t, err)

	_, _, err = linaje.Advance(deployment.ContentID{})
	require.Error(t, err)
}

func TestLineage_LaIgualdadIncluyeLaCabeza(t *testing.T) {
	contenido := materialBase(t).componer(t)
	linaje, err := deployment.LineageOf(contenido, deployment.DeploymentID{})
	require.NoError(t, err)

	avanzado, _, err := linaje.Advance(contenido.ID())
	require.NoError(t, err)

	assert.True(t, linaje.Equals(linaje))
	assert.False(t, linaje.Equals(avanzado),
		"el mismo ambiente en dos momentos distintos de su historia")
}
