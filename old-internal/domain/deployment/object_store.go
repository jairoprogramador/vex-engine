package deployment

import "context"

// ObjectStore guarda la INTENCIÓN CONGELADA, direccionada por su contenido.
//
// # Write-once, y la idempotencia no es una comodidad
//
// La dirección de un objeto ES su `content_id`, así que dos objetos que caen en
// la misma dirección son bit a bit el mismo objeto. Escribir uno que ya existe
// no es un conflicto: es la misma intención declarada otra vez, que es
// exactamente lo que pasa cuando se vuelve a desplegar sin tocar nada. Por eso
// `Put` es idempotente y NO devuelve «ya existía» — quien lo llama no tiene
// ninguna decisión que tomar con esa respuesta.
//
// Lo que sí es un error es que el archivo exista con OTRO contenido: eso sería
// una colisión de sha256 o un archivo corrupto, y las dos cosas invalidan el
// direccionamiento entero. El almacén lo detecta o no según lo que pueda leer;
// lo que no hace nunca es sobrescribir.
//
// # No tiene lectura todavía, y es a propósito
//
// La consulta —`record show`, `record verify`— es de la spec 22, que es quien
// sabrá qué preguntas hace. Un puerto con un método sin llamador es una decisión
// tomada sin el caso de uso delante.
//
// El puerto vive aquí y no en `pipeline`, que es quien lo consume, por lo mismo
// que `state.Records` vive en `state`: habla de tipos de este paquete, y su
// segundo consumidor —la consulta de la 22— no está en la cadena de pipeline.
type ObjectStore interface {
	Put(ctx *context.Context, content Content, metadata ObjectMetadata) error
}

// ObjectMetadata es lo que acompaña al objeto SIN entrar en su identidad.
//
// Existe como tipo propio, y no como dos campos más de `Content`, para que la
// frontera sea del compilador y no de la disciplina: lo que está aquí no puede
// colarse en el hash. La regla es la de la spec 08 sin excepción —**la huella
// identifica, el commit documenta**—, y su consecuencia es que dos árboles
// idénticos con distinto commit de origen tienen el mismo `content_id`.
//
// Como el objeto es write-once, lo que queda escrito es el metadato del PRIMER
// intento que compuso ese contenido. Es correcto: el objeto describe una
// intención, y de qué commit salió la primera vez que se declaró es un dato
// sobre esa declaración, no sobre cada repetición. Qué commit corrió cada vez lo
// dicen los hechos del intento, que sí son uno por ejecución.
type ObjectMetadata struct {
	// ProjectCommit es el HEAD del árbol del proyecto.
	ProjectCommit string

	// PipelineCommit es el HEAD del clon del pipelinecode. Se leía y se
	// descartaba hasta la spec 18 §5.3.
	PipelineCommit string
}

// LineageStore guarda LA CABEZA de la historia de un ambiente: el último
// `deployment_id` registrado para un par (sujeto, destino).
//
// # Una cabeza por ambiente, no por operación
//
// `Lineage` es `(sujeto, destino)` y no lleva la operación: pedir `supply` y
// pedir `deploy` sobre `prod` caen en la misma historia, en el orden en que
// ocurrieron. Partirla por operación produciría dos líneas de tiempo paralelas
// sobre un ambiente real donde los efectos se acumulan en una sola.
//
// # Por qué NO vive en el área de trabajo del motor
//
// Es la tienda que hay que LEER antes de ejecutar —sin la cabeza no hay `parent`
// y sin `parent` no hay posición—, así que se comporta como `state/` y `cache/`
// y no como `objects/` y `events/`: va directa al destino configurado, no a un
// búfer que se empuja al terminar (spec 21 §5.1). Si viviera en el área de
// trabajo, cada máquina efímera empezaría con una historia vacía y dos
// ejecuciones del mismo contenido derivarían el MISMO `deployment_id` —el de
// padre cero—, que es la única cosa que una posición en la historia no puede
// hacer.
type LineageStore interface {
	// Head rehidrata el linaje de un ambiente.
	//
	// La AUSENCIA no es un error: es «este ambiente no tiene historia todavía», y
	// devuelve un linaje con la cabeza a cero, del que sale un primer
	// `deployment_id` sin padre. Un archivo ILEGIBLE sí lo es, y por la misma
	// asimetría que el registro de step (spec 11 §5.6): seguir sin la cabeza
	// derivaría una posición que ya existe, y `deployment_id` es permanente.
	Head(ctx *context.Context, subject Subject, destination Destination) (Lineage, error)

	// Save publica la cabeza nueva. Es la única tienda del motor que SUSTITUYE:
	// el linaje no es una historia de archivos —esa son los eventos—, es un
	// puntero al último.
	Save(ctx *context.Context, lineage Lineage) error
}
