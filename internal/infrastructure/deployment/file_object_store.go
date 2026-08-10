package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	domDeployment "github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/persistence"
)

// ObjectsDirName es el directorio de los objetos, dentro del área de trabajo del
// motor y dentro del destino cuando la spec 21 los empuja.
//
// Exportado por lo mismo que `EventsDirName`: el layout tiene un dueño, y es
// quien lo escribe. El empuje construye OTRO `FileObjectStore` con esta misma
// raíz bajo el destino, lo que le da write-once, idempotencia y la comparación
// canónica sin reimplementar ninguna de las tres.
const ObjectsDirName = "objects"

const (
	// shardLen son los caracteres del hash que dan nombre al directorio
	// intermedio. Es el mismo reparto que el índice de caché, y por la misma
	// razón: un destino compartido por varios proyectos acumularía miles de
	// archivos en un solo directorio.
	shardLen = 2

	// objectFileExt es `.json` y no `.gob`: un objeto de despliegue es lo que un
	// humano abre para saber qué se pretendía hacer, y lo que un ingestor de otra
	// plataforma tiene que poder leer sin este binario.
	objectFileExt = ".json"
)

var _ domDeployment.ObjectStore = (*FileObjectStore)(nil)

// FileObjectStore guarda un archivo por objeto, direccionado por contenido:
//
//	<base>/<cnt-v1>/<2 primeros del hash>/<resto>.json
//
// La versión de la regla encabeza la ruta por lo mismo que en el índice: el día
// que `cnt-v2` exista, los objetos de las dos reglas conviven sin que ninguno
// pise al otro, y `ls` dice cuál es cuál.
//
// La ruta NO lleva ni el proyecto, ni el ambiente, ni la operación: los tres
// están DENTRO del hash. Es lo que hace que «¿esta misma configuración ya se
// desplegó alguna vez, en cualquier ambiente?» se responda con un `stat`.
type FileObjectStore struct {
	basePath string
	writer   persistence.AtomicFileWriter
}

func NewFileObjectStore(basePath string) domDeployment.ObjectStore {
	return &FileObjectStore{
		basePath: basePath,
		writer:   persistence.NewAtomicFileWriter(),
	}
}

func (s *FileObjectStore) filePath(id domDeployment.ContentID) string {
	hash := id.Hash()
	return filepath.Join(s.basePath, id.Version(), hash[:shardLen], hash[shardLen:]+objectFileExt)
}

// Put escribe el objeto si no estaba.
//
// # Write-once, e idempotente
//
// La dirección ES el contenido, así que un objeto que ya existe es este mismo
// objeto declarado otra vez —lo que pasa cada vez que se re-despliega sin tocar
// nada—. No se reescribe: no habría nada que cambiar salvo el metadato, y el
// metadato del primer intento es tan cierto como el del segundo. Devolver «ya
// existía» tampoco serviría de nada: quien llama no tiene ninguna decisión que
// tomar con esa respuesta.
//
// # Lo que sí es un error: el mismo hash con otro contenido
//
// Sería una colisión de sha256 o un archivo corrupto, y las dos cosas invalidan
// el direccionamiento entero. Se comprueba comparando la forma CANÓNICA, que es
// el material del que salió el hash, y no el archivo byte a byte: dos versiones
// del serializador pueden escribir el mismo objeto con otro orden de campos sin
// que nada esté mal.
//
// Un archivo existente ILEGIBLE es un error y no una ausencia: sobrescribirlo
// sería tapar una corrupción de una tienda permanente.
func (s *FileObjectStore) Put(
	_ *context.Context,
	content domDeployment.Content,
	metadata domDeployment.ObjectMetadata) error {

	id := content.ID()
	if id.IsZero() {
		return errors.New("file object store: el contenido no tiene identidad")
	}

	path := s.filePath(id)
	stored, found, err := s.read(path)
	if err != nil {
		return err
	}
	if found {
		if stored.Canonical != content.Canonical() {
			return fmt.Errorf(
				"file object store: %s ya existe con OTRO contenido: el objeto es "+
					"direccionado por contenido y no se sobrescribe", path)
		}
		return nil
	}

	dto := ToFileObjectDTO(content, metadata)
	err = s.writer.Write(path, func(out io.Writer) error {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(dto)
	})
	if err != nil {
		return fmt.Errorf("file object store: escribir %s: %w", path, err)
	}
	return nil
}

func (s *FileObjectStore) read(path string) (FileObjectDTO, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileObjectDTO{}, false, nil
		}
		return FileObjectDTO{}, false, fmt.Errorf("file object store: abrir %s: %w", path, err)
	}
	defer file.Close()

	var dto FileObjectDTO
	if err := json.NewDecoder(file).Decode(&dto); err != nil {
		return FileObjectDTO{}, false, fmt.Errorf(
			"file object store: decodificar %s: %w", path, err)
	}
	return dto, true, nil
}
