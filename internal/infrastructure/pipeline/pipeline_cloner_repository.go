package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	gogit "github.com/go-git/go-git/v5"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/persistence"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

const (
	// clonesMarkerDirName es donde viven las marcas de clonación. Va FUERA del
	// árbol del clon a propósito: un archivo dentro cambiaría la huella del
	// pipelinecode, que es material de la identidad del despliegue (spec 18
	// §5.1). El punto inicial lo mantiene fuera del camino de un `ls`.
	clonesMarkerDirName = ".clones"

	markerFileExt = ".json"

	// clonesTempPrefix nombra el directorio donde se clona antes de publicar.
	// Existe por la mitad que hace posible el fallback: el clon viejo no se
	// borra hasta que hay uno nuevo entero (§5.4).
	clonesTempPrefix = ".clonando-"

	// fileCloneMarkerSchemaVersion versiona la forma de la marca.
	fileCloneMarkerSchemaVersion = 1

	markerTimeLayout = time.RFC3339Nano
)

// FileCloneMarkerDTO es lo que se sabe de un clon que ya está en disco: de dónde
// salió, de qué ref, y cuándo se trajo.
//
// La edad no se deduce del `mtime` del directorio porque el `mtime` lo mueve
// cualquiera —una lectura con `atime` relativo, un antivirus, un `chmod`— y de
// esa edad depende que el motor decida hablar con el remoto o no. Un dato del
// que depende una decisión se escribe, no se infiere.
type FileCloneMarkerDTO struct {
	SchemaVersion int    `json:"schema_version"`
	URL           string `json:"url"`
	Ref           string `json:"ref"`
	ClonedAt      string `json:"cloned_at"`
}

var _ domPipeline.PipelineClonerRepository = (*PipelineClonerRepository)(nil)

// PipelineClonerRepository resuelve la fuente del pipelinecode con las TRES
// políticas de la spec 18 §5.4:
//
//	reutilizar    hay un clon de esta url y esta ref dentro de su ventana
//	clonar        no lo hay, o se salió de la ventana
//	usar el viejo el remoto no respondió y hay un clon que sí
//
// Hasta aquí hacía `os.RemoveAll` + clon `depth 1` en CADA ejecución, sin
// ventana y sin fallback: si el remoto no respondía, la ejecución fallaba aunque
// hubiera un clon perfectamente válido en disco (BL-27). Y como el borrado iba
// ANTES del clon, un fallo de red dejaba además el directorio vacío, así que la
// ejecución siguiente tampoco tenía a qué caer.
type PipelineClonerRepository struct {
	repositoryBasePath string
	manifests          domPipeline.ManifestRepository
	clock              shared.Clock
	writer             persistence.AtomicFileWriter
}

func NewPipelineClonerRepository(
	repositoryBasePath string,
	manifests domPipeline.ManifestRepository,
	clock shared.Clock) domPipeline.PipelineClonerRepository {

	return &PipelineClonerRepository{
		repositoryBasePath: repositoryBasePath,
		manifests:          manifests,
		clock:              clock,
		writer:             persistence.NewAtomicFileWriter(),
	}
}

func (r *PipelineClonerRepository) Clone(
	ctx *context.Context, urlPipeline, refPipeline string) (domPipeline.PipelineSource, error) {

	pipelineName := utils.GetDirNameFromUrl(urlPipeline)
	localPath := filepath.Join(r.repositoryBasePath, pipelineName)

	existing := r.existingClone(ctx, localPath, urlPipeline, refPipeline)

	if existing.usable && existing.age < existing.window {
		head, err := headHashOf(localPath)
		if err != nil {
			// Un clon del que no se puede leer el HEAD no es un clon reutilizable:
			// se cae al camino normal, que lo sustituye.
			return r.fresh(ctx, urlPipeline, refPipeline, localPath, pipelineName, cloneState{})
		}
		return domPipeline.PipelineSource{
			LocalPath: localPath,
			HeadHash:  head,
			Origin:    domPipeline.CloneReused,
			Age:       existing.age,
		}, nil
	}

	return r.fresh(ctx, urlPipeline, refPipeline, localPath, pipelineName, existing)
}

// cloneState es lo que se sabe del clon que ya estaba: si sirve, cuánto vale y
// cuánto tiene.
type cloneState struct {
	usable bool
	age    time.Duration
	window time.Duration
}

// existingClone mira el disco y responde si hay un clon de ESTA url y ESTA ref
// del que fiarse, y durante cuánto.
//
// La ventana sale del `vexpipeline.yaml` DEL CLON QUE YA ESTÁ, que es la única
// copia disponible antes de decidir si hablar con el remoto. No es una
// contradicción: la pregunta que la ventana responde es «¿me vale lo que tengo?»,
// y quien la contesta es lo que se tiene.
func (r *PipelineClonerRepository) existingClone(
	ctx *context.Context, localPath, urlPipeline, refPipeline string) cloneState {

	info, err := os.Stat(localPath)
	if err != nil || !info.IsDir() {
		return cloneState{}
	}

	marker, found := r.readMarker(localPath)
	if !found || marker.URL != urlPipeline || marker.Ref != refPipeline {
		// Sin marca no se sabe la edad, y una edad desconocida no se puede
		// comparar con una ventana ni contar en un `stale_clone_used`. Se trata
		// como si no hubiera clon: la corrida deja marca y la siguiente ya la
		// tiene.
		return cloneState{}
	}

	clonedAt, err := time.Parse(markerTimeLayout, marker.ClonedAt)
	if err != nil {
		return cloneState{}
	}

	age := r.clock.Now().Sub(clonedAt)
	if age < 0 {
		// Un clon del futuro es un reloj movido, no un clon fresco.
		age = 0
	}

	window := domPipeline.DefaultCloneWindow
	if manifest, err := r.manifests.Get(ctx, localPath); err == nil {
		window = manifest.CloneWindow()
	}

	return cloneState{usable: true, age: age, window: window}
}

// fresh trae el pipelinecode del remoto, y si no puede, se queda con lo que hay.
//
// El clon nuevo se construye en un directorio aparte y se publica con un rename:
// hasta que está entero, el que ya estaba sigue en su sitio. Es lo que convierte
// «el remoto no responde» en un aviso en vez de en un fallo.
func (r *PipelineClonerRepository) fresh(
	ctx *context.Context,
	urlPipeline, refPipeline, localPath, pipelineName string,
	existing cloneState) (domPipeline.PipelineSource, error) {

	tempPath := filepath.Join(r.repositoryBasePath, clonesTempPrefix+pipelineName)
	if err := os.RemoveAll(tempPath); err != nil {
		return domPipeline.PipelineSource{}, fmt.Errorf(
			"pipeline cloner repository: eliminar clon a medias '%s': %w", tempPath, err)
	}
	if err := os.MkdirAll(tempPath, 0o750); err != nil {
		return domPipeline.PipelineSource{}, fmt.Errorf(
			"pipeline cloner repository: crear base '%s': %w", tempPath, err)
	}

	cloneErr := cloneWithRef(*ctx, urlPipeline, refPipeline, tempPath, 1)
	if cloneErr != nil {
		_ = os.RemoveAll(tempPath)
		return r.fallback(localPath, urlPipeline, refPipeline, existing, cloneErr)
	}

	if err := os.RemoveAll(localPath); err != nil {
		_ = os.RemoveAll(tempPath)
		return domPipeline.PipelineSource{}, fmt.Errorf(
			"pipeline cloner repository: eliminar ruta previa '%s': %w", localPath, err)
	}
	if err := os.Rename(tempPath, localPath); err != nil {
		_ = os.RemoveAll(tempPath)
		return domPipeline.PipelineSource{}, fmt.Errorf(
			"pipeline cloner repository: publicar clon en '%s': %w", localPath, err)
	}

	head, err := headHashOf(localPath)
	if err != nil {
		return domPipeline.PipelineSource{}, fmt.Errorf(
			"pipeline cloner repository: leer el HEAD de '%s': %w", localPath, err)
	}

	// La marca se escribe DESPUÉS del clon: una marca sin clon detrás haría que
	// la corrida siguiente reutilizara un directorio que no existe.
	if err := r.writeMarker(localPath, urlPipeline, refPipeline); err != nil {
		return domPipeline.PipelineSource{}, err
	}

	return domPipeline.PipelineSource{
		LocalPath: localPath,
		HeadHash:  head,
		Origin:    domPipeline.CloneFresh,
	}, nil
}

// fallback es la tercera política: el remoto no respondió y hay un clon viejo.
//
// Devuelve la procedencia `stale`, que es lo que obliga al handler a emitir el
// hecho. Sin clon al que caer, el error del remoto sube tal cual: no hay nada
// con lo que ejecutar y decirlo es todo lo que se puede hacer.
func (r *PipelineClonerRepository) fallback(
	localPath, urlPipeline, refPipeline string,
	existing cloneState,
	cloneErr error) (domPipeline.PipelineSource, error) {

	if !existing.usable {
		return domPipeline.PipelineSource{}, fmt.Errorf(
			"pipeline cloner repository: clonar ref %q de %s: %w", refPipeline, urlPipeline, cloneErr)
	}

	head, err := headHashOf(localPath)
	if err != nil {
		return domPipeline.PipelineSource{}, fmt.Errorf(
			"pipeline cloner repository: clonar ref %q de %s (%v) y el clon local de '%s' "+
				"tampoco se puede usar: %w", refPipeline, urlPipeline, cloneErr, localPath, err)
	}

	return domPipeline.PipelineSource{
		LocalPath: localPath,
		HeadHash:  head,
		Origin:    domPipeline.CloneStale,
		Age:       existing.age,
	}, nil
}

func (r *PipelineClonerRepository) markerPath(localPath string) string {
	return filepath.Join(
		filepath.Dir(localPath), clonesMarkerDirName, filepath.Base(localPath)+markerFileExt)
}

func (r *PipelineClonerRepository) readMarker(localPath string) (FileCloneMarkerDTO, bool) {
	data, err := os.ReadFile(r.markerPath(localPath))
	if err != nil {
		return FileCloneMarkerDTO{}, false
	}

	var dto FileCloneMarkerDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return FileCloneMarkerDTO{}, false
	}
	if dto.SchemaVersion != fileCloneMarkerSchemaVersion {
		return FileCloneMarkerDTO{}, false
	}
	return dto, true
}

// writeMarker deja constancia de qué se trajo y cuándo.
//
// Su ausencia o su ilegibilidad NO son un error de ejecución en la LECTURA —se
// vuelve a clonar, que es la dirección segura—, pero no poder ESCRIBIRLA sí lo
// es: significa que la ventana no funcionaría nunca y el síntoma sería que «no
// funciona» sin que nada falle, que es el modo de fallo que la spec 18 §5.4
// nombra explícitamente.
func (r *PipelineClonerRepository) writeMarker(localPath, urlPipeline, refPipeline string) error {
	dto := FileCloneMarkerDTO{
		SchemaVersion: fileCloneMarkerSchemaVersion,
		URL:           urlPipeline,
		Ref:           refPipeline,
		ClonedAt:      r.clock.Now().UTC().Format(markerTimeLayout),
	}

	path := r.markerPath(localPath)
	err := r.writer.Write(path, func(out io.Writer) error {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(dto)
	})
	if err != nil {
		return fmt.Errorf("pipeline cloner repository: escribir la marca %s: %w", path, err)
	}
	return nil
}

// headHashOf lee el commit del clon.
//
// Ya se leía —`verifyWorkingTreeMatchesRef` abre el repo y pide `Head()`— y se
// descartaba. Aquí deja de descartarse: viaja como metadato del objeto de
// despliegue, nunca como identidad (spec 18 §5.3).
func headHashOf(localPath string) (string, error) {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return "", fmt.Errorf("abrir el clon '%s': %w", localPath, err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("leer el HEAD de '%s': %w", localPath, err)
	}
	return head.Hash().String(), nil
}
