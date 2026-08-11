package deployment_test

// El almacén de objetos, con la disciplina del contract test de la spec 02.
//
// Lo que se prueba aquí es la regla de vida de la tienda, que es la que la
// distingue de las otras tres: direccionada por contenido, write-once, y
// permanente. Un objeto no se sobrescribe nunca; volver a declarar la misma
// intención no es un conflicto.

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domDeployment "github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
	infraDeployment "github.com/jairoprogramador/vex-engine/internal/infrastructure/deployment"
)

func contenidoDePrueba(t *testing.T, operacion, ambiente string) domDeployment.Content {
	t.Helper()

	huella := func(version, digito string) fingerprint.Fingerprint {
		f, err := fingerprint.Parse(version + ":" + strings.Repeat(digito, 32))
		require.NoError(t, err)
		return f
	}

	subject, err := domDeployment.NewSubject("https://vex.test/acme/demo-app")
	require.NoError(t, err)
	operation, err := domDeployment.NewOperation(operacion)
	require.NoError(t, err)
	destination, err := domDeployment.NewDestination(ambiente)
	require.NoError(t, err)
	source, err := domDeployment.NewSource(
		huella(fingerprint.Version, "11"), huella(fingerprint.Version, "22"))
	require.NoError(t, err)
	format, err := domDeployment.NewFormat(1, false)
	require.NoError(t, err)

	declaracion, err := domStep.NewLiteralDeclaration("registry_prefix", "vexsand")
	require.NoError(t, err)
	stepContent, err := domDeployment.NewStepContent(
		"02-supply",
		domStep.NoStepConfig(),
		huella(fingerprint.DeclarationVersion, "33"),
		[]domStep.VariableDeclaration{declaracion})
	require.NoError(t, err)

	content, err := domDeployment.NewContent(
		subject, operation, destination, source, format,
		[]domDeployment.StepContent{stepContent})
	require.NoError(t, err)
	return content
}

func archivosDe(t *testing.T, base string) []string {
	t.Helper()
	rutas := make([]string, 0, 4)
	err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		rutas = append(rutas, filepath.ToSlash(rel))
		return nil
	})
	require.NoError(t, err)
	return rutas
}

// La dirección ES el contenido: la ruta lleva la versión de la regla y el hash,
// y ni el proyecto, ni el ambiente, ni la operación. Es lo que hace que «¿esta
// misma configuración ya se desplegó alguna vez, en cualquier ambiente?» se
// responda con un `stat`.
func TestFileObjectStore_LaRutaSaleDelContenido(t *testing.T) {
	base := t.TempDir()
	ctx := context.Background()
	store := infraDeployment.NewFileObjectStore(base)

	content := contenidoDePrueba(t, "supply", "sand")
	require.NoError(t, store.Put(&ctx, content, domDeployment.ObjectMetadata{}))

	rutas := archivosDe(t, base)
	require.Len(t, rutas, 1)

	id := content.ID()
	assert.Equal(t,
		id.Version()+"/"+id.Hash()[:2]+"/"+id.Hash()[2:]+".json",
		rutas[0],
		"la versión de la regla encabeza la ruta: el día que exista `cnt-v2` los dos conviven")
}

// Declarar dos veces la misma intención no es un conflicto: es lo que pasa cada
// vez que se re-despliega sin tocar nada. Se escribe un archivo, no dos, y no se
// reescribe.
func TestFileObjectStore_WriteOnceEsIdempotente(t *testing.T) {
	base := t.TempDir()
	ctx := context.Background()
	store := infraDeployment.NewFileObjectStore(base)

	content := contenidoDePrueba(t, "supply", "sand")
	require.NoError(t, store.Put(&ctx, content, domDeployment.ObjectMetadata{
		ProjectCommit: "aaa", PipelineCommit: "bbb"}))

	// El metadato del SEGUNDO intento es otro y no gana: el objeto describe una
	// intención, y de qué commit salió la primera vez que se declaró es un dato
	// sobre esa declaración.
	require.NoError(t, store.Put(&ctx, content, domDeployment.ObjectMetadata{
		ProjectCommit: "zzz", PipelineCommit: "yyy"}))

	rutas := archivosDe(t, base)
	require.Len(t, rutas, 1)

	var dto infraDeployment.FileObjectDTO
	data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(rutas[0])))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &dto))

	assert.Equal(t, "aaa", dto.Source.ProjectCommit)
	assert.Equal(t, "bbb", dto.Source.PipelineCommit)
	assert.Equal(t, content.ID().String(), dto.ContentID)
	assert.Equal(t, content.Canonical(), dto.Canonical,
		"la forma canónica viaja entera: es lo que un tercero tiene que poder reproducir")
}

// Dos contenidos distintos en la misma dirección invalidarían el
// direccionamiento entero. No se sobrescribe: se dice.
func TestFileObjectStore_LaMismaDireccionConOtroContenidoEsUnError(t *testing.T) {
	base := t.TempDir()
	ctx := context.Background()
	store := infraDeployment.NewFileObjectStore(base)

	content := contenidoDePrueba(t, "supply", "sand")
	require.NoError(t, store.Put(&ctx, content, domDeployment.ObjectMetadata{}))

	// Se falsifica la colisión escribiendo otra forma canónica en la dirección de
	// ésta: un sha256 no colisiona a mano, pero un archivo corrupto sí llega.
	ruta := filepath.Join(base, filepath.FromSlash(archivosDe(t, base)[0]))
	data, err := os.ReadFile(ruta)
	require.NoError(t, err)
	var dto infraDeployment.FileObjectDTO
	require.NoError(t, json.Unmarshal(data, &dto))
	dto.Canonical = "otra cosa"
	falsificado, err := json.Marshal(dto)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(ruta, falsificado, 0o644))

	err = store.Put(&ctx, content, domDeployment.ObjectMetadata{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "OTRO contenido")
}

// Ilegible es ERROR y no ausencia: sobrescribir taparía una corrupción de una
// tienda permanente.
func TestFileObjectStore_UnObjetoIlegibleNoSeSobrescribe(t *testing.T) {
	base := t.TempDir()
	ctx := context.Background()
	store := infraDeployment.NewFileObjectStore(base)

	content := contenidoDePrueba(t, "supply", "sand")
	require.NoError(t, store.Put(&ctx, content, domDeployment.ObjectMetadata{}))

	ruta := filepath.Join(base, filepath.FromSlash(archivosDe(t, base)[0]))
	require.NoError(t, os.WriteFile(ruta, []byte("{no es json"), 0o644))

	err := store.Put(&ctx, content, domDeployment.ObjectMetadata{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decodificar")
}

// Dos ambientes son dos objetos: el destino entra en la identidad.
func TestFileObjectStore_DosDestinosSonDosObjetos(t *testing.T) {
	base := t.TempDir()
	ctx := context.Background()
	store := infraDeployment.NewFileObjectStore(base)

	require.NoError(t, store.Put(&ctx,
		contenidoDePrueba(t, "supply", "sand"), domDeployment.ObjectMetadata{}))
	require.NoError(t, store.Put(&ctx,
		contenidoDePrueba(t, "supply", "prod"), domDeployment.ObjectMetadata{}))

	assert.Len(t, archivosDe(t, base), 2)
}
