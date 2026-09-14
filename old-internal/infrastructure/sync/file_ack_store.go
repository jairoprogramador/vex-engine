package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	domRecord "github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	domSync "github.com/jairoprogramador/vex-engine/old-internal/domain/sync"
	"github.com/jairoprogramador/vex-engine/old-internal/infrastructure/persistence"
	recordInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/record"
)

// AckDirName es donde vive el puntero de confirmación, dentro del ÁREA DE
// TRABAJO del motor.
//
// # Por qué en el área de trabajo y no en el destino
//
// Porque perderlo no cuesta nada: degrada a reenvío total, que es idempotente
// (§5.3). Ponerlo en el destino lo haría depender de que el volumen esté montado
// para optimizar el envío hacia ese mismo volumen, que es circular.
//
// # Y por qué al lado de `events/` y no dentro
//
// Porque **lo que se sincroniza tiene que seguir siendo enumerable**: `objects/`
// y `events/`, y nada más. Un puntero de transporte dentro de la tira que
// transporta acabaría empujándose a sí mismo.
const AckDirName = "ack"

// ackFileExt es `.json` frente al `.jsonl` de la tira: es un valor, no una
// historia. El nombre del archivo es el mismo —el `execution_id`— así que las
// dos rutas se leen juntas.
const ackFileExt = ".json"

// ackSchemaVersion versiona la forma del archivo.
const ackSchemaVersion = 1

var _ domSync.AckStore = (*FileAckStore)(nil)

// FileAckDTO es la forma externa del puntero.
//
// Lleva el despliegue y la ejecución además de la posición, y no por
// redundancia: un archivo suelto tiene que poder decir de qué es. La ruta ya lo
// dice, pero una ruta se copia y un archivo que sólo contiene un número no se
// puede diagnosticar.
type FileAckDTO struct {
	SchemaVersion int    `json:"schema_version"`
	DeploymentID  string `json:"deployment_id"`
	ExecutionID   string `json:"execution_id"`
	Seq           uint64 `json:"seq"`
}

// FileAckStore guarda un archivo por tira:
//
//	<staging>/ack/<dep-v1>/<hash>/<execution_id>.json
//
// El área de trabajo resuelta es una raíz ESTABLE y no `staging/<execution_id>/`
// como proponía el plan: la separación por intento la da el layout de la spec 17
// §5.8, que ya lleva el `execution_id` en el nombre del archivo. Dos intentos
// nunca comparten puntero por la misma razón por la que no comparten tira.
type FileAckStore struct {
	basePath string
	writer   persistence.AtomicFileWriter
}

func NewFileAckStore(basePath string) domSync.AckStore {
	return &FileAckStore{
		basePath: basePath,
		writer:   persistence.NewAtomicFileWriter(),
	}
}

func (s *FileAckStore) filePath(stream domRecord.EventStream) string {
	return filepath.Join(
		s.basePath,
		recordInfra.StreamDir(stream.Deployment),
		stream.ExecutionID+ackFileExt)
}

// Last responde «no consta» ante cualquier duda.
//
// Ausente, ilegible o de otra versión de esquema dan todos la posición cero, y
// eso NO es tapar un error: la respuesta correcta ante la duda es reenviar la
// tira entera, que es idempotente. Es la asimetría opuesta a la del almacén de
// registros (spec 11 §5.6), donde un archivo ilegible sí es un error — allí la
// duda no se puede resolver sin arriesgar un recurso duplicado en la nube, y
// aquí lo único que se arriesga es ancho de banda.
func (s *FileAckStore) Last(
	_ *context.Context, stream domRecord.EventStream) (domRecord.Seq, error) {

	if err := stream.Validate(); err != nil {
		return domRecord.Seq{}, err
	}

	file, err := os.Open(s.filePath(stream))
	if err != nil {
		return domRecord.Seq{}, nil
	}
	defer file.Close()

	var dto FileAckDTO
	if err := json.NewDecoder(file).Decode(&dto); err != nil {
		return domRecord.Seq{}, nil
	}
	if dto.SchemaVersion != ackSchemaVersion || dto.Seq == 0 {
		return domRecord.Seq{}, nil
	}

	seq, err := domRecord.NewSeq(dto.Seq)
	if err != nil {
		return domRecord.Seq{}, nil
	}
	return seq, nil
}

// Save publica el puntero con el escritor atómico de la spec 02: un `ack` a
// medias sería un número truncado, que es peor que ninguno.
func (s *FileAckStore) Save(
	_ *context.Context, stream domRecord.EventStream, confirmed domRecord.Seq) error {

	if err := stream.Validate(); err != nil {
		return err
	}
	if confirmed.IsZero() {
		return fmt.Errorf("file ack store: la posición cero no es una confirmación")
	}

	dto := FileAckDTO{
		SchemaVersion: ackSchemaVersion,
		DeploymentID:  stream.Deployment.String(),
		ExecutionID:   stream.ExecutionID,
		Seq:           confirmed.Position(),
	}

	path := s.filePath(stream)
	err := s.writer.Write(path, func(out io.Writer) error {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(dto)
	})
	if err != nil {
		return fmt.Errorf("file ack store: escribir %s: %w", path, err)
	}
	return nil
}
