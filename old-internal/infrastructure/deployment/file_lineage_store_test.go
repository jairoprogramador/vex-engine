package deployment_test

// La cabeza de la historia de un ambiente.
//
// Es la única tienda del motor que SUSTITUYE, y las tres propiedades que se
// prueban aquí son las que hacen que eso no sea una contradicción con «los
// registros no se sobrescriben»: la historia son los objetos y los eventos; esto
// es un puntero al último.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domDeployment "github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	infraDeployment "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/deployment"
)

func ambienteDePrueba(t *testing.T, nombre string) (domDeployment.Subject, domDeployment.Destination) {
	t.Helper()
	subject, err := domDeployment.NewSubject("https://vex.test/acme/demo-app")
	require.NoError(t, err)
	destination, err := domDeployment.NewDestination(nombre)
	require.NoError(t, err)
	return subject, destination
}

// Un ambiente sin historia no es un error: es el caso normal la primera vez, y
// de ahí sale un primer `deployment_id` sin padre.
func TestFileLineageStore_SinHistoriaLaCabezaEsCero(t *testing.T) {
	ctx := context.Background()
	store := infraDeployment.NewFileLineageStore(t.TempDir())
	subject, destination := ambienteDePrueba(t, "sand")

	lineage, err := store.Head(&ctx, subject, destination)

	require.NoError(t, err)
	assert.True(t, lineage.IsEmpty())
	assert.False(t, lineage.IsZero(), "el linaje existe; lo que no tiene es cabeza")
}

// La historia avanza: cada contenido cuelga del anterior, y el MISMO contenido
// dos veces da dos posiciones distintas. Es la diferencia entre las dos
// identidades y la razón de que la decisión de saltar un step no pueda consultar
// `deployment_id`.
func TestFileLineageStore_LaCabezaAvanzaYSeRelee(t *testing.T) {
	base := t.TempDir()
	ctx := context.Background()
	store := infraDeployment.NewFileLineageStore(base)
	subject, destination := ambienteDePrueba(t, "sand")

	content := contenidoDePrueba(t, "supply", "sand")

	primero, err := store.Head(&ctx, subject, destination)
	require.NoError(t, err)
	primero, id1, err := primero.Advance(content.ID())
	require.NoError(t, err)
	require.NoError(t, store.Save(&ctx, primero))

	segundo, err := store.Head(&ctx, subject, destination)
	require.NoError(t, err)
	require.True(t, segundo.Head().Equals(id1), "la cabeza releída es la que se guardó")

	_, id2, err := segundo.Advance(content.ID())
	require.NoError(t, err)

	assert.False(t, id2.Equals(id1),
		"el mismo contenido dos veces cae en dos posiciones distintas: la segunda cuelga de la primera")
}

// Dos ambientes son dos historias, y no se pisan.
func TestFileLineageStore_CadaAmbienteTieneLaSuya(t *testing.T) {
	base := t.TempDir()
	ctx := context.Background()
	store := infraDeployment.NewFileLineageStore(base)

	subject, sand := ambienteDePrueba(t, "sand")
	_, prod := ambienteDePrueba(t, "prod")

	lineage, err := store.Head(&ctx, subject, sand)
	require.NoError(t, err)
	lineage, _, err = lineage.Advance(contenidoDePrueba(t, "supply", "sand").ID())
	require.NoError(t, err)
	require.NoError(t, store.Save(&ctx, lineage))

	deProd, err := store.Head(&ctx, subject, prod)
	require.NoError(t, err)
	assert.True(t, deProd.IsEmpty(), "avanzar `sand` no le da historia a `prod`")
}

// Ilegible es ERROR y no ausencia: continuar sin la cabeza derivaría la posición
// de un primer despliegue para un ambiente que ya tiene historia, y esa
// identidad es permanente.
func TestFileLineageStore_UnaCabezaIlegibleEsUnError(t *testing.T) {
	base := t.TempDir()
	ctx := context.Background()
	store := infraDeployment.NewFileLineageStore(base)
	subject, destination := ambienteDePrueba(t, "sand")

	lineage, err := store.Head(&ctx, subject, destination)
	require.NoError(t, err)
	lineage, _, err = lineage.Advance(contenidoDePrueba(t, "supply", "sand").ID())
	require.NoError(t, err)
	require.NoError(t, store.Save(&ctx, lineage))

	rutas := archivosDe(t, base)
	require.Len(t, rutas, 1)
	require.NoError(t, os.WriteFile(
		filepath.Join(base, filepath.FromSlash(rutas[0])), []byte("{roto"), 0o644))

	_, err = store.Head(&ctx, subject, destination)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decodificar")
}

// Un linaje sin cabeza no se guarda: su archivo diría lo mismo que su ausencia
// y, a diferencia de ella, podría fallar al leerse.
func TestFileLineageStore_NoSeGuardaUnLinajeSinCabeza(t *testing.T) {
	base := t.TempDir()
	ctx := context.Background()
	store := infraDeployment.NewFileLineageStore(base)
	subject, destination := ambienteDePrueba(t, "sand")

	lineage, err := domDeployment.NewLineage(subject, destination, domDeployment.DeploymentID{})
	require.NoError(t, err)

	require.Error(t, store.Save(&ctx, lineage))
	assert.Empty(t, archivosDe(t, base))
}
