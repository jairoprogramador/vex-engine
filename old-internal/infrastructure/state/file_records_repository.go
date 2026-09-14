// Package state es el almacén append-only de registros de step en disco.
//
// Es la tienda NO desechable del motor: `cache/` se puede borrar entero sin
// perder un hecho, y esto no, porque aquí viven los identificadores de recursos
// que existen de verdad en la nube (spec 11 §2).
package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	domState "github.com/jairoprogramador/vex-engine/old-internal/domain/state"
	"github.com/jairoprogramador/vex-engine/old-internal/infrastructure/persistence"
	"github.com/jairoprogramador/vex-engine/old-internal/infrastructure/utils"
)

// recordFileExt es `.json`: ver el comentario de FileStepRecordDTO.
const recordFileExt = ".json"

var _ domState.Records = (*FileRecordsRepository)(nil)

// FileRecordsRepository guarda un archivo por registro:
//
//	<base>/<subject>/<scope…>/<step_id>/<record_id>.json
//
// La ruta es POSICIÓN, no contenido: es la diferencia con el índice de
// `infrastructure/cache`, cuya ruta sale del hash. Aquí la pregunta no es «¿qué
// contenido es éste?» sino «¿qué ha pasado en este sitio?», y por eso el último
// registro se obtiene ordenando nombres de archivo.
//
// Con append-only el rename atómico de la spec 02 deja de ser una sustitución:
// cada archivo se escribe UNA vez y no se toca nunca más, lo que elimina la
// clase entera de D4 en esta tienda — no hay archivo a medio reescribir porque
// no se reescribe ninguno.
type FileRecordsRepository struct {
	basePath string
	writer   persistence.AtomicFileWriter
}

func NewFileRecordsRepository(basePath string) domState.Records {
	return &FileRecordsRepository{
		basePath: basePath,
		writer:   persistence.NewAtomicFileWriter(),
	}
}

// keyDir traduce la clave del dominio a una ruta.
//
// `subject` es la url del proyecto y se convierte al mismo nombre de directorio
// —último segmento + 8 caracteres del sha256 de la url— que usa el resto del
// almacenamiento: dos proyectos con el mismo nombre corto no colisionan.
//
// El ámbito aporta UNO o DOS segmentos (`project`, o `environment/<nombre>`).
// La forma lógica lleva dos puntos y la ruta no, porque `:` es ilegal en rutas
// de Windows (spec 11 §5.4).
func (r *FileRecordsRepository) keyDir(key domState.Key) string {
	segments := []string{r.basePath, utils.GetDirNameFromUrl(key.Subject())}
	segments = append(segments, key.Scope().Segments()...)
	segments = append(segments, key.StepID())
	return filepath.Join(segments...)
}

// Append escribe el registro y **falla si ya existía uno con ese
// identificador**.
//
// La comprobación no defiende de una colisión de ULID —80 bits de entropía por
// milisegundo—, sino de un defecto del llamador: es la única forma de que
// «append-only» sea una propiedad comprobada en el borde de la tienda y no una
// costumbre de quien la usa.
func (r *FileRecordsRepository) Append(_ *context.Context, key domState.Key, record domState.StepRecord) error {
	if key.IsZero() {
		return errors.New("file records repository: clave vacía")
	}
	if record.ID().IsZero() {
		return errors.New("file records repository: registro sin record_id")
	}

	path := filepath.Join(r.keyDir(key), record.ID().String()+recordFileExt)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf(
			"file records repository: el registro %s ya existe y un registro no se sobrescribe", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("file records repository: comprobar %s: %w", path, err)
	}

	dto := ToFileStepRecordDTO(record)
	err := r.writer.Write(path, func(out io.Writer) error {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(dto)
	})
	if err != nil {
		return fmt.Errorf("file records repository: escribir %s: %w", path, err)
	}
	return nil
}

// Last lee UN archivo: el del mayor `record_id`.
//
// Los ULID ordenan lexicográficamente en orden temporal y `os.ReadDir` devuelve
// las entradas ordenadas por nombre, así que el último es el último elemento de
// la lista. No se abre ningún otro — que es la propiedad que la spec 11 §7 pide
// comprobar.
func (r *FileRecordsRepository) Last(_ *context.Context, key domState.Key) (domState.StepRecord, bool, error) {
	if key.IsZero() {
		return domState.StepRecord{}, false, errors.New("file records repository: clave vacía")
	}

	dir := r.keyDir(key)
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Ausencia NO es error: es «este step nunca corrió aquí», y aguas arriba
		// significa ejecutar.
		if errors.Is(err, os.ErrNotExist) {
			return domState.StepRecord{}, false, nil
		}
		return domState.StepRecord{}, false, fmt.Errorf(
			"file records repository: listar %s: %w", dir, err)
	}

	name := ""
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != recordFileExt {
			continue
		}
		if entry.Name() > name {
			name = entry.Name()
		}
	}
	if name == "" {
		return domState.StepRecord{}, false, nil
	}

	return r.read(filepath.Join(dir, name))
}

// Get lee el archivo cuyo nombre ES el `record_id`.
//
// La ruta se compone, no se busca: el identificador viene de un `RecordID` ya
// validado —26 símbolos del alfabeto de Crockford—, así que no puede contener un
// separador ni una travesía de directorios. Es lo que permite que aquí no haya
// una segunda validación de ruta que envejecería por separado de la del value
// object.
//
// Ausencia NO es error: el registro apuntado —por un evento viejo, por un índice
// que sobrevivió a lo que apuntaba— pudo desaparecer o no haberse escrito nunca,
// y quien pregunta necesita distinguirlo de una corrupción.
func (r *FileRecordsRepository) Get(
	_ *context.Context, key domState.Key, recordID domState.RecordID) (domState.StepRecord, bool, error) {

	if key.IsZero() {
		return domState.StepRecord{}, false, errors.New("file records repository: clave vacía")
	}
	if recordID.IsZero() {
		return domState.StepRecord{}, false, errors.New("file records repository: record_id vacío")
	}

	path := filepath.Join(r.keyDir(key), recordID.String()+recordFileExt)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domState.StepRecord{}, false, nil
		}
		return domState.StepRecord{}, false, fmt.Errorf(
			"file records repository: comprobar %s: %w", path, err)
	}
	return r.read(path)
}

// read decodifica un registro ya localizado.
//
// Ilegible es ERROR, no ausencia (spec 11 §5.6). Incluido el archivo vacío: un
// registro se escribe entero de una vez, así que cero bytes sólo puede ser
// corrupción, y leerla como «no consta» perdería en silencio la pista de un
// recurso real.
func (r *FileRecordsRepository) read(path string) (domState.StepRecord, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return domState.StepRecord{}, false, fmt.Errorf(
			"file records repository: abrir %s: %w", path, err)
	}
	defer file.Close()

	var dto FileStepRecordDTO
	if err := json.NewDecoder(file).Decode(&dto); err != nil {
		return domState.StepRecord{}, false, fmt.Errorf(
			"file records repository: decodificar %s: %w", path, err)
	}

	record, err := dto.ToDomain()
	if err != nil {
		return domState.StepRecord{}, false, fmt.Errorf(
			"file records repository: %s: %w", path, err)
	}
	return record, true, nil
}
