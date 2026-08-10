package sync

import (
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/record"
)

// Batch es una SELECCIÓN, no una transacción ni una unidad de trabajo.
//
// Esa distinción es lo que permite que reenviarlo sea seguro (§5.1'): un lote no
// afirma «esto se envía junto o no se envía», dice «esto es lo pendiente desde
// aquí». De ahí que perder el `ack` no rompa nada — sólo agranda la selección.
//
// # Lo que lleva, y por qué el objeto viaja entero
//
//   - La TIRA y desde qué posición: `stream` + `from`. Los hechos no viajan en
//     el lote porque el empuje LEE ARCHIVOS —`record.EventSink` es de escritura
//     y no hay lector publicado todavía; lo publica la spec 22—, así que lo que
//     el lote transporta es la dirección de la tira, no su contenido.
//   - La INTENCIÓN: `content` + `metadata`. Aquí sí viaja el material entero, y
//     no su `content_id`, porque **lo que se empuja de un objeto es su MATERIAL,
//     no su hash** (spec 17). El destino tiene que poder recomponer la forma
//     canónica y verificarla; una regla que sólo se puede comprobar por su hash
//     no se puede depurar cuando dos implementaciones discrepan.
//
// El objeto se re-empuja en cada lote y eso NO es desperdicio: es write-once
// direccionado por contenido, así que el segundo envío es un `stat` en el
// destino. Llevarlo sólo en el primer lote costaría un punto de control —«¿ya
// mandé el objeto?»— y la propiedad que hace simple a todo esto es justamente
// que **la correctitud no depende de ningún punto de control** (§4).
type Batch struct {
	stream   record.EventStream
	from     record.Seq
	content  deployment.Content
	metadata deployment.ObjectMetadata
}

// NewBatch compone la selección.
//
// `from` en cero es legítimo y es el caso normal del primer empuje: significa
// «desde el principio de la tira». Lo que no es legítimo es un lote sin tira o
// sin intención — un empuje que no sabe qué manda ni a nombre de qué despliegue.
func NewBatch(
	stream record.EventStream,
	from record.Seq,
	content deployment.Content,
	metadata deployment.ObjectMetadata) (Batch, error) {

	if err := stream.Validate(); err != nil {
		return Batch{}, err
	}
	if content.ID().IsZero() {
		return Batch{}, fmt.Errorf(
			"sync: el lote de %s no declara ninguna intención", stream.ExecutionID)
	}
	return Batch{stream: stream, from: from, content: content, metadata: metadata}, nil
}

// Stream es la tira a la que pertenece lo pendiente.
func (b Batch) Stream() record.EventStream { return b.stream }

// From es la última posición ya confirmada por el destino. Se empuja lo MAYOR
// que ella; el cero significa «nada confirmado todavía».
func (b Batch) From() record.Seq { return b.from }

// Content es la intención congelada que este intento declaró.
func (b Batch) Content() deployment.Content { return b.content }

// Metadata es lo que acompaña al objeto sin entrar en su identidad.
func (b Batch) Metadata() deployment.ObjectMetadata { return b.metadata }

// IsZero indica que no hay lote.
func (b Batch) IsZero() bool { return b.stream.IsZero() }
