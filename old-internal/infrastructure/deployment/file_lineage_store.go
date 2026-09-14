package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	domDeployment "github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/old-internal/infrastructure/persistence"
	"github.com/jairoprogramador/vex-engine/old-internal/infrastructure/utils"
)

// fileLineageSchemaVersion versiona la forma del archivo del linaje.
const fileLineageSchemaVersion = 1

// lineageFileExt es `.json`, por lo mismo que el resto de las tiendas.
const lineageFileExt = ".json"

// FileLineageDTO es la cabeza de la historia de un ambiente.
//
// Lleva el sujeto y el destino DENTRO además de en la ruta: la ruta abrevia la
// url del proyecto a `<último segmento><8 del sha256>`, así que el archivo tiene
// que poder decir de qué proyecto es sin que nadie invierta un hash.
type FileLineageDTO struct {
	SchemaVersion int    `json:"schema_version"`
	Subject       string `json:"subject"`
	Destination   string `json:"destination"`

	// Head es el último `deployment_id` registrado, en su forma canónica con
	// prefijo. Vacío no puede llegar a disco: un linaje sin cabeza no se guarda,
	// porque su ausencia ya significa eso mismo.
	Head string `json:"head"`
}

var _ domDeployment.LineageStore = (*FileLineageStore)(nil)

// FileLineageStore guarda un archivo por ambiente:
//
//	<base>/<subject>/<destino>.json
//
// El proyecto se convierte al mismo nombre de directorio que usa el resto del
// almacenamiento —`utils.GetDirNameFromUrl`— para que dos proyectos con el mismo
// nombre corto no colisionen, y para que un `ls` del destino enseñe la misma
// forma en `state/`, en `cache/` y aquí.
//
// **Es la única tienda del motor que sustituye.** No es una contradicción con
// «los registros no se sobrescriben»: la historia son los objetos y los eventos,
// que sí son append-only; esto es un PUNTERO al último, y un puntero que no se
// mueve no es un puntero. Se publica con rename atómico, así que un lector nunca
// ve una cabeza a medias.
type FileLineageStore struct {
	basePath string
	writer   persistence.AtomicFileWriter
}

func NewFileLineageStore(basePath string) domDeployment.LineageStore {
	return &FileLineageStore{
		basePath: basePath,
		writer:   persistence.NewAtomicFileWriter(),
	}
}

func (s *FileLineageStore) filePath(
	subject domDeployment.Subject, destination domDeployment.Destination) string {

	return filepath.Join(
		s.basePath,
		utils.GetDirNameFromUrl(subject.String()),
		destination.String()+lineageFileExt)
}

// Head rehidrata el linaje del ambiente.
//
// AUSENCIA no es error: es el caso normal la primera vez, y significa «este
// ambiente no tiene historia todavía». De ahí sale un primer `deployment_id` sin
// padre, que no es un hueco — la raíz aporta la cadena vacía al material, que
// ninguna identidad real puede producir.
//
// ILEGIBLE sí lo es, y es la misma asimetría del registro de step (spec 11
// §5.6): continuar sin la cabeza derivaría la posición de un primer despliegue
// para un ambiente que ya tiene historia, y esa identidad es permanente.
func (s *FileLineageStore) Head(
	_ *context.Context,
	subject domDeployment.Subject,
	destination domDeployment.Destination) (domDeployment.Lineage, error) {

	if subject.IsZero() || destination.IsZero() {
		return domDeployment.Lineage{}, errors.New("file lineage store: linaje sin ambiente")
	}

	path := s.filePath(subject, destination)
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domDeployment.NewLineage(subject, destination, domDeployment.DeploymentID{})
		}
		return domDeployment.Lineage{}, fmt.Errorf("file lineage store: abrir %s: %w", path, err)
	}
	defer file.Close()

	var dto FileLineageDTO
	if err := json.NewDecoder(file).Decode(&dto); err != nil {
		return domDeployment.Lineage{}, fmt.Errorf(
			"file lineage store: decodificar %s: %w", path, err)
	}
	if dto.SchemaVersion != fileLineageSchemaVersion {
		return domDeployment.Lineage{}, fmt.Errorf(
			"file lineage store: esquema de linaje %d no soportado en %s (este binario lee el %d)",
			dto.SchemaVersion, path, fileLineageSchemaVersion)
	}

	head, err := domDeployment.ParseDeploymentID(dto.Head)
	if err != nil {
		return domDeployment.Lineage{}, fmt.Errorf("file lineage store: %s: %w", path, err)
	}
	return domDeployment.NewLineage(subject, destination, head)
}

// Save publica la cabeza nueva.
//
// Un linaje sin cabeza no se escribe: su archivo diría lo mismo que su ausencia
// y, a diferencia de ella, podría fallar al leerse.
func (s *FileLineageStore) Save(_ *context.Context, lineage domDeployment.Lineage) error {
	if lineage.IsZero() {
		return errors.New("file lineage store: linaje sin abrir")
	}
	if lineage.IsEmpty() {
		return errors.New("file lineage store: un linaje sin cabeza no se guarda")
	}

	path := s.filePath(lineage.Subject(), lineage.Destination())
	dto := FileLineageDTO{
		SchemaVersion: fileLineageSchemaVersion,
		Subject:       lineage.Subject().String(),
		Destination:   lineage.Destination().String(),
		Head:          lineage.Head().String(),
	}

	err := s.writer.Write(path, func(out io.Writer) error {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(dto)
	})
	if err != nil {
		return fmt.Errorf("file lineage store: escribir %s: %w", path, err)
	}
	return nil
}
