package deployment

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	// contentHeader encabeza el material del objeto. Existe para que un
	// `content_id` no pueda coincidir con el hash de otra cosa que se le parezca.
	contentHeader = "vex-content/" + ContentIDVersion

	// canonicalLineSep une las líneas del material. Sin salto final.
	canonicalLineSep = "\n"

	// canonicalFieldSep separa los campos DENTRO de una línea. Como todos los
	// valores van entrecomillados con la sintaxis de Go, que escapa todo carácter
	// de control, ningún valor puede simular un separador ni un salto de línea:
	// la regla es inyectiva por construcción.
	canonicalFieldSep = "\x1e"
)

// Content es la INTENCIÓN CONGELADA: qué se pretende ejecutar, sin una sola
// circunstancia dentro.
//
// Es un agregado inmutable cuya identidad ES su contenido. No tiene setters, no
// tiene ciclo de vida, y esa rigidez es la funcionalidad: es lo que permite
// calcular la identidad ANTES de ejecutar, y de ahí cuelgan `vex plan` y la
// afirmación «este mismo contenido ya funcionó, luego el fallo es transitorio».
//
// # Lo que NO entra, y es la mitad del valor
//
//	timestamp   actor   runner   parent
//
// Sin esa exclusión el direccionamiento por contenido no existe: dos ejecuciones
// idénticas darían identidades distintas y toda comparación se caería. El
// `parent` no falta por olvido —es lo que convierte a `Content` en intención y a
// `DeploymentID` en posición—, y esa separación es la decisión de la que cuelga
// todo lo demás.
//
// # Y en `content_id` entra la DECLARACIÓN de cómo se resuelve un parámetro,
// nunca su valor
//
// Escrito aquí y escrito en el código (`StepContent.canonical`). Dos ejecuciones
// que resuelven valores distintos para la misma declaración producen el mismo
// `content_id`; dos pipelinecode que sólo difieren en un `from` producen
// identidades distintas.
type Content struct {
	subject     Subject
	operation   Operation
	destination Destination
	source      Source
	format      Format
	steps       []StepContent
}

// NewContent compone el objeto y VALIDA que su material esté completo.
//
// Material incompleto ⇒ error, nunca identidad degradada (spec 17 §5.1, regla
// d). La razón es la misma que en `cache.NewCacheKey` y aquí es más grave: una
// identidad compuesta con un hueco es perfectamente válida y COLISIONA con la de
// cualquier otro material al que le falte lo mismo. En el caché esa colisión
// cuesta un paso saltado; aquí cuesta dos despliegues distintos con la misma
// identidad, y `content_id` es permanente.
//
// Que la validación esté en el constructor es lo que permite que `ID()` sea
// total: un `Content` que existe es un `Content` completo.
//
// `steps` va en el ORDEN DE EJECUCIÓN, que es el del prefijo de orden de los
// directorios. No se reordena: ejecutar `01-test` antes que `02-supply` es parte
// de lo que se pretende hacer, así que dos órdenes distintos son dos intenciones
// distintas.
func NewContent(
	subject Subject,
	operation Operation,
	destination Destination,
	source Source,
	format Format,
	steps []StepContent) (Content, error) {

	if subject.IsZero() {
		return Content{}, fmt.Errorf("deployment: el contenido no tiene sujeto")
	}
	if operation.IsZero() {
		return Content{}, fmt.Errorf("deployment: el contenido no tiene operación")
	}
	if destination.IsZero() {
		return Content{}, fmt.Errorf("deployment: el contenido no tiene destino")
	}
	if source.IsZero() {
		return Content{}, fmt.Errorf("deployment: el contenido no tiene fuente")
	}
	if format.IsZero() {
		return Content{}, fmt.Errorf("deployment: el contenido no dice con qué formato se leyó")
	}
	// Cero steps es un ERROR y no un objeto vacío: una operación que no ejecuta
	// nada no es una intención, y su identidad colisionaría con la de cualquier
	// otra operación vacía del mismo ambiente.
	if len(steps) == 0 {
		return Content{}, fmt.Errorf("deployment: el contenido no tiene ningún step")
	}

	seen := make(map[string]bool, len(steps))
	for _, stepContent := range steps {
		if stepContent.IsZero() {
			return Content{}, fmt.Errorf("deployment: el contenido tiene un step sin material")
		}
		if seen[stepContent.StepID()] {
			return Content{}, fmt.Errorf(
				"deployment: el contenido repite el step %q", stepContent.StepID())
		}
		seen[stepContent.StepID()] = true
	}

	return Content{
		subject:     subject,
		operation:   operation,
		destination: destination,
		source:      source,
		format:      format,
		steps:       slices.Clone(steps),
	}, nil
}

func (c Content) Subject() Subject         { return c.subject }
func (c Content) Operation() Operation     { return c.operation }
func (c Content) Destination() Destination { return c.destination }
func (c Content) Source() Source           { return c.source }
func (c Content) Format() Format           { return c.format }

// Steps es el material de los steps, en orden de ejecución.
func (c Content) Steps() []StepContent { return slices.Clone(c.steps) }

// ProjectScopedSteps es el sub-bloque cuyo material es INDEPENDIENTE DEL
// DESTINO (spec 17 §5.1, spec 13).
//
// Es la primera vez que `Content` tiene una parte que no depende del ambiente, y
// existe como consulta para que la propiedad sea comprobable: dos `Content` que
// sólo difieren en `Destination` tienen el mismo material en este sub-bloque.
// Queda declarado para que nadie meta el ambiente ahí más adelante.
func (c Content) ProjectScopedSteps() []StepContent {
	shared := make([]StepContent, 0, len(c.steps))
	for _, stepContent := range c.steps {
		if stepContent.IsProjectScoped() {
			shared = append(shared, stepContent)
		}
	}
	return shared
}

// IsComplete dice si el material del objeto está completo.
//
// Incompleto significa una cosa concreta: el pipelinecode no trae
// `vexpipeline.yaml`, así que no declara de dónde salen sus variables y una
// producida por otro step aparece en el material como si no viniera de ninguna
// parte (spec 18 §5.5). El objeto se compone igual y se emite MARCADO: «este
// material está incompleto» es un hecho; omitirlo sería perder la historia, y
// emitirlo como completo sería mentir.
func (c Content) IsComplete() bool { return c.format.IsDeclared() }

// ID es la identidad del contenido: el sha256 de su forma canónica.
//
// Es total —no devuelve error— porque `NewContent` ya validó que el material
// esté completo: un `Content` que existe se puede identificar. El valor cero
// devuelve el `ContentID` cero, que es explícitamente «sin identidad».
func (c Content) ID() ContentID {
	if c.subject.IsZero() {
		return ContentID{}
	}
	sum := sha256.Sum256([]byte(c.Canonical()))
	id, err := newContentID(hex.EncodeToString(sum[:]))
	if err != nil {
		// Inalcanzable: el hash de un sha256 en hexadecimal siempre valida. Se
		// devuelve el valor cero en vez de entrar en pánico porque el dominio no
		// entra en pánico (convención del repositorio).
		return ContentID{}
	}
	return id
}

// Canonical es la forma normativa del objeto (`SPEC-CONTENT-v1.md` §5).
//
// Está EXPUESTA a propósito: es lo que un tercero —el backend de la spec 26—
// tiene que poder reproducir bit a bit, y una regla que sólo se puede comprobar
// por su hash no se puede depurar cuando dos implementaciones discrepan.
func (c Content) Canonical() string {
	lines := []string{
		contentHeader,
		strconv.Quote(c.subject.String()),
		strconv.Quote(c.operation.String()),
		strconv.Quote(c.destination.String()),
		// Las dos huellas entran por su forma canónica COMPLETA, con prefijo. De
		// ahí sale que un salto a `v2` de la regla del árbol cambie todos los
		// `content_id` sin una línea de código extra — y aquí eso no es una
		// comodidad, es un requisito: si compusiera sobre el hash pelado, el
		// mismo árbol identificado con dos reglas distintas daría el mismo
		// `content_id`, que es el único fallo que un registro direccionado por
		// contenido no puede permitirse.
		strconv.Quote(c.source.Project().String()),
		strconv.Quote(c.source.Pipeline().String()),
		strconv.Quote(strconv.Itoa(c.format.SchemaVersion())),
		strconv.Quote(strconv.FormatBool(c.format.IsDeclared())),
		// El número de steps encabeza el bloque para que la regla se pueda leer
		// sin adivinar dónde termina: no hace falta para la inyectividad —las
		// comillas ya la garantizan— y sí para que un tercero la reimplemente sin
		// mirar este código.
		strconv.Itoa(len(c.steps)),
	}
	for _, stepContent := range c.steps {
		lines = append(lines, stepContent.canonical()...)
	}
	return strings.Join(lines, canonicalLineSep)
}
