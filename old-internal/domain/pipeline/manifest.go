package pipeline

import (
	"context"
	"fmt"
	"time"
)

// Las versiones del formato del pipelinecode que el motor entiende (spec 14
// §5.1, cierra D-A16).
//
// UNA versión para TODO el formato, no una por archivo. `commands.yaml` también
// evolucionó —la spec 13 le añadió `config.yaml` al lado, la 15 le añadirá
// campos— y versiones por archivo serían N negociaciones independientes para un
// formato que se publica como una unidad.
const (
	// SchemaVersion1 es lo que hay hoy en disco: literales solamente.
	SchemaVersion1 = 1

	// SchemaVersion2 añade la gramática de `resolve` (spec 14 §5.2).
	SchemaVersion2 = 2
)

// Manifest es `vexpipeline.yaml`, el contrato EXPLÍCITO del formato.
//
// Es la pieza que faltaba para poder evolucionar el pipelinecode sin romper a
// nadie en silencio: hasta ahora `variables/<ambiente>/<paso>.yaml` no tenía
// ningún campo donde poner un origen NI versión con la que negociar uno, así que
// añadir gramática era o romper a todo el mundo o adivinar.
type Manifest struct {
	declared      bool
	schemaVersion int

	// cloneWindow es cuánto vale un clon de este pipelinecode antes de volver a
	// traerlo (spec 18 §5.4). Cero significa «no lo declara» y de ahí sale
	// `DefaultCloneWindow`, no un cero literal: una ventana de cero re-clonaría
	// en cada corrida, que es lo contrario de lo que la omisión quiere decir.
	//
	// NO entra en `deployment.Format` ni en ninguna identidad, y no por descuido:
	// es un parámetro OPERATIVO —cada cuánto conviene mirar el remoto— y no una
	// afirmación sobre qué se pretende ejecutar. Lo que sí lo cubre es la huella
	// del árbol del pipelinecode, que lo incluye como a cualquier otro archivo.
	cloneWindow time.Duration
}

// NoManifest es el pipelinecode SIN `vexpipeline.yaml`: versión 1, literales
// solamente, `resolve` prohibido. El motor lo ejecuta con normalidad.
//
// La ausencia no es un error y no puede serlo: los tres templates reales y todo
// pipelinecode escrito hasta hoy están en esa situación. Lo que sí ocurre es que
// la spec 18 marca su `content_id` como INCOMPLETO de forma explícita en vez de
// emitirlo como si fuera completo — es la disciplina del registro aplicada al
// propio contrato: «este pipelinecode no declara orígenes» es un hecho, «esta
// identidad es completa» sería una conclusión falsa.
func NoManifest() Manifest {
	return Manifest{declared: false, schemaVersion: SchemaVersion1}
}

// NewManifest es el manifiesto que alguien SÍ escribió. Una versión fuera de las
// conocidas es un error que nombra las que hay: un motor viejo leyendo un formato
// nuevo tiene que decirlo, no interpretarlo a medias.
func NewManifest(schemaVersion int) (Manifest, error) {
	return NewManifestWithCloneWindow(schemaVersion, 0)
}

// NewManifestWithCloneWindow es el manifiesto con su ventana de reutilización
// del clon declarada (spec 18 §5.4). Una ventana negativa es un error: la
// omisión ya tiene su forma —el cero, que significa «usa la de por defecto»— y
// un número negativo sólo puede ser un `-24h` escrito a mano.
func NewManifestWithCloneWindow(schemaVersion int, cloneWindow time.Duration) (Manifest, error) {
	if cloneWindow < 0 {
		return Manifest{}, fmt.Errorf(
			"declara 'clone_window: %s', que no es una ventana de tiempo", cloneWindow)
	}
	switch schemaVersion {
	case SchemaVersion1, SchemaVersion2:
		return Manifest{
			declared:      true,
			schemaVersion: schemaVersion,
			cloneWindow:   cloneWindow,
		}, nil
	case 0:
		return Manifest{}, fmt.Errorf(
			"no declara 'schema_version': se espera %d o %d", SchemaVersion1, SchemaVersion2)
	default:
		return Manifest{}, fmt.Errorf(
			"'schema_version: %d' no la entiende este motor: se espera %d o %d",
			schemaVersion, SchemaVersion1, SchemaVersion2)
	}
}

// IsDeclared dice si el pipelinecode trae `vexpipeline.yaml`.
func (m Manifest) IsDeclared() bool { return m.declared }

// SchemaVersion es la versión vigente del formato: la declarada, o la 1.
func (m Manifest) SchemaVersion() int { return m.schemaVersion }

// AllowsDeclaredSources dice si este pipelinecode puede usar `resolve`.
func (m Manifest) AllowsDeclaredSources() bool { return m.schemaVersion >= SchemaVersion2 }

// CloneWindow es cuánto vale un clon de este pipelinecode: la declarada, o la de
// por defecto.
//
// El default vive AQUÍ y no en el clonador porque la pregunta «¿cuánto vale un
// clon?» es del pipelinecode aunque no la conteste: quien no declara nada está
// aceptando la respuesta del motor, y tener un solo sitio donde esa respuesta se
// escribe es lo que impide que dos capas contesten distinto.
func (m Manifest) CloneWindow() time.Duration {
	if m.cloneWindow <= 0 {
		return DefaultCloneWindow
	}
	return m.cloneWindow
}

// DeclaresCloneWindow dice si alguien la escribió.
func (m Manifest) DeclaresCloneWindow() bool { return m.cloneWindow > 0 }

// ManifestRepository lee el manifiesto de la raíz del pipelinecode.
//
// La ausencia del archivo NO es un error —devuelve `NoManifest()`—, igual que la
// del `config.yaml` de un step. Un archivo PRESENTE que no declara una versión
// conocida SÍ lo es: presente es una intención de declarar, y tratarlo como
// ausente sería adivinar cuál.
type ManifestRepository interface {
	Get(ctx *context.Context, pipelineLocalPath string) (Manifest, error)
}
