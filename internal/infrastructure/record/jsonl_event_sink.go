package record

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	domDeployment "github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	domRecord "github.com/jairoprogramador/vex-engine/internal/domain/record"
)

// EventsDirName es el directorio de las tiras dentro del área de trabajo del
// motor, y dentro del destino cuando la spec 21 las empuja.
//
// Está exportado porque el LAYOUT TIENE UN DUEÑO: quien escribe las tiras. El
// cableado lo usa para construir este sink y el empuje lo usa para saber de
// dónde lee y a dónde escribe; que cada uno declarara su propia constante sería
// tener el mismo nombre escrito en tres sitios y esperar que nadie lo mueva.
const EventsDirName = "events"

const (
	// eventsFileExt es JSON Lines: un hecho por línea, y el archivo sigue siendo
	// legible aunque la última esté a medias. Un JSON array no lo sería, y la
	// interrupción es el caso normal de este motor, no el excepcional.
	eventsFileExt = ".jsonl"

	// eventsDirPerm y eventsFilePerm son los mismos que el resto de las tiendas.
	eventsDirPerm  os.FileMode = 0o755
	eventsFilePerm os.FileMode = 0o644
)

var _ domRecord.EventSink = (*JSONLEventSink)(nil)

// JSONLEventSink anexa los hechos de un intento a su archivo:
//
//	<base>/<deployment_id>/<execution_id>.jsonl
//
// # Por qué NO usa el escritor atómico
//
// El resto de las tiendas publica con rename porque sustituye contenido; esto
// ANEXA. Un rename por hecho reescribiría el archivo entero cada vez —O(n²)
// sobre una tira que crece— y, peor, perdería la propiedad que justifica el
// modelo: lo escrito hasta el instante de una muerte dura tiene que seguir ahí.
// Un `O_APPEND` con `Sync` da exactamente eso.
//
// La atomicidad que sí hace falta es la de la LÍNEA, y por eso cada hecho se
// serializa entero en memoria antes de tocar el archivo: sobre POSIX un
// `write(2)` en `O_APPEND` por debajo de `PIPE_BUF` no se intercala con otro, y
// una línea a medias haría ilegible el resto de la tira.
//
// # Un archivo por (despliegue, ejecución), no por intento
//
// `events/<attempt>.jsonl` reintroducía el fallo que el layout evita: dos
// intentos de dos despliegues distintos con el mismo número escriben el mismo
// archivo, y `attempt` se deriva de un conteo sin atomicidad. `execution_id` ya
// es único por construcción (decisión I-2, spec 17 §5.8).
type JSONLEventSink struct {
	basePath string
}

func NewJSONLEventSink(basePath string) domRecord.EventSink {
	return &JSONLEventSink{basePath: basePath}
}

func (s *JSONLEventSink) filePath(stream domRecord.EventStream) string {
	return filepath.Join(s.basePath, StreamRelPath(stream))
}

// StreamRelPath es la ruta de la tira de un intento RELATIVA a la raíz de
// `events/`, sea la del área de trabajo o la del destino.
//
// Existe exportada porque el empuje de la spec 21 lee ARCHIVOS y no el puerto
// —`EventSink.Append` es de escritura, y el lector lo publica la spec 22—, así
// que necesita componer la misma ruta que este sink escribe. Que la componga
// esta función y no una copia en el sincronizador es lo que impide que las dos
// mitades del mismo layout se separen.
func StreamRelPath(stream domRecord.EventStream) string {
	return filepath.Join(StreamDir(stream.Deployment), stream.ExecutionID+eventsFileExt)
}

// StreamDir traduce el `deployment_id` a un tramo de ruta.
//
// La forma canónica es `dep-v1:<hash>` y los dos puntos son ilegales en rutas de
// Windows —el mismo motivo por el que el ámbito del estado aporta dos segmentos
// en vez de uno (spec 11 §5.4)—. Se parte en `<versión>/<hash>` en vez de
// sustituir el separador: así la versión de la regla encabeza el directorio y
// dos reglas distintas no mezclan sus tiras.
//
// Lo usa además el puntero de confirmación de la spec 21, que se dirige por la
// misma tira con otra extensión.
func StreamDir(id domDeployment.DeploymentID) string {
	return filepath.Join(id.Version(), id.Hash())
}

// Append escribe los hechos, en el orden en que llegan.
//
// Escribir CERO hechos no es un error ni abre el archivo: el emisor llama con lo
// que tenga retenido, y no tener nada retenido es lo normal.
func (s *JSONLEventSink) Append(
	_ *context.Context, stream domRecord.EventStream, events []domRecord.Event) error {

	if err := stream.Validate(); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}

	// Los hechos se serializan ANTES de abrir el archivo: uno inválido no puede
	// dejar la tira con la mitad de un bloque escrita.
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for _, event := range events {
		dto, err := ToJSONLEventDTO(event)
		if err != nil {
			return err
		}
		if err := encoder.Encode(dto); err != nil {
			return fmt.Errorf("jsonl event sink: serializar el hecho '%s': %w", event.Type(), err)
		}
	}

	path := s.filePath(stream)
	if err := os.MkdirAll(filepath.Dir(path), eventsDirPerm); err != nil {
		return fmt.Errorf("jsonl event sink: crear directorio de %s: %w", path, err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, eventsFilePerm)
	if err != nil {
		return fmt.Errorf("jsonl event sink: abrir %s: %w", path, err)
	}

	if _, err := file.Write(buffer.Bytes()); err != nil {
		_ = file.Close()
		return fmt.Errorf("jsonl event sink: escribir en %s: %w", path, err)
	}
	// El `Sync` no es prudencia: el caso que este archivo existe para cubrir es
	// el de la máquina que muere, y un hecho que sólo está en el page cache no
	// sobrevive a eso. El `Close` va sin `defer` desnudo por lo mismo que en el
	// escritor atómico (spec 02 §5.4): en el camino de escritura su error es el
	// que revela que los datos no llegaron.
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("jsonl event sink: sincronizar %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("jsonl event sink: cerrar %s: %w", path, err)
	}
	return nil
}
