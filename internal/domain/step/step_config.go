package step

import (
	"context"
	"fmt"
)

// StepConfig es lo que un step declara SOBRE SÍ MISMO en
// `steps/NN-nombre/config.yaml`.
//
// Lleva DOS cosas, juntas en el archivo y separadas como conceptos a propósito:
// el ámbito es DÓNDE vive el estado, la regla es CUÁNDO ese estado deja de ser
// válido. Son ortogonales, y mezclarlas haría más difícil razonar sobre cada una
// o añadir reglas sin tocar la definición de ámbito. Se intentó, en un momento
// del diseño, tratar «el estado del ámbito cambió» como si fuera parte del
// concepto de ámbito; es un error, y por eso son dos claves y no una.
//
// Que vayan en el MISMO archivo es economía de conceptos: un `policy/<step>.yaml`
// aparte sería un tercer archivo por step para describir una sola unidad, y la
// unidad es el step. La separación que sí hace falta —la conceptual— se consigue
// con dos claves.
//
// «Un step, un ámbito» es una invariante de AGREGADO y no una convención de
// estilo: es lo que permite que un step tenga una identidad y una huella. Un
// agregado con dos criterios de cambio no es un agregado — por eso el tipo lleva
// un `Scope` y no una lista, y por eso un step que necesita los dos se parte en
// dos steps, que el orden numérico ya secuencia (spec 13 §5.2).
type StepConfig struct {
	declared bool
	scope    Scope
	rules    RuleSet
}

// NewStepConfig es la configuración de un step que SÍ declara. El ámbito vacío
// no puede llegar aquí: quien lo construye ya pasó por `NewScope`.
//
// El conjunto de reglas SÍ puede ir vacío, y no es un caso degradado: es un step
// que declara dónde vive su estado y no declara nada que lo invalide, así que se
// ejecuta siempre y no persiste nada (§5.5).
func NewStepConfig(scope Scope, rules RuleSet) (StepConfig, error) {
	if scope.IsZero() {
		return StepConfig{}, fmt.Errorf("step: la configuración no declara ámbito")
	}
	return StepConfig{declared: true, scope: scope, rules: rules}, nil
}

// NoStepConfig es el step SIN `config.yaml`: no declara ámbito, y de ahí se
// sigue todo lo demás (spec 13 §5.3).
//
//	Se ejecuta siempre, y no escribe registro de estado.
//
// Ejecutar siempre es el default seguro de todo el catálogo —ejecutar de más
// nunca produce un despliegue que no ocurrió—. Y NO ESCRIBIR es lo que impide
// inventarle un ámbito: si el motor le asignara `environment` por defecto,
// estaríamos en la misma deducción implícita que esta spec retira, sólo que en
// el otro archivo.
//
// Sus variables siguen viajando en el mapa acumulado de la corrida, así que los
// steps posteriores las ven igual que hoy; lo que no ocurre es que crucen de una
// ejecución a la siguiente. Tiene precedente exacto: un step sin comandos es
// `skipped{no_commands}` y tampoco persiste estado (spec 04 §5.3, D-A12).
func NoStepConfig() StepConfig { return StepConfig{} }

// IsDeclared dice si el step tiene `config.yaml`.
func (c StepConfig) IsDeclared() bool { return c.declared }

// Scope es el ámbito declarado, o el ámbito cero si no hay `config.yaml`.
func (c StepConfig) Scope() Scope { return c.scope }

// Rules son las reglas de re-ejecución declaradas, o el conjunto vacío.
func (c StepConfig) Rules() RuleSet { return c.rules }

// Remembers dice si este step deja constancia de lo que hizo.
//
// Hacen falta las DOS cosas —un ámbito donde recordarse y algo que comprobar— y
// las dos ausencias son la misma decisión escrita dos veces:
//
//	sin `config.yaml`  no declara ámbito, luego no hay DÓNDE (spec 13 §5.3)
//	sin `rules`        no hay AFIRMACIÓN que guardar (spec 15 §5.5)
//
// Tiene un tercer precedente exacto: un step sin comandos es
// `skipped{no_commands}` y tampoco persiste estado (spec 04 §5.3).
//
// La pérdida que esto acepta, y conviene tenerla escrita: un step con ámbito y
// `rules: []` produce variables que NO cruzan de una ejecución a la siguiente,
// así que nadie puede leerlas con `resolve: state`. Se acepta porque el step se
// ejecuta siempre —las vuelve a producir en cada corrida, y el mapa acumulado
// las lleva a los steps posteriores— y porque la alternativa era escribir un
// registro por corrida, para siempre, que nadie puede revivir ni leer.
func (c StepConfig) Remembers() bool { return c.declared && !c.rules.IsEmpty() }

// StepConfigRepository lee lo que un step declara sobre sí mismo.
//
// La ausencia del archivo NO es un error: es un step que no declara ámbito, y el
// repositorio devuelve `NoStepConfig()`. Un archivo PRESENTE cuyo `scope` falta
// o está fuera del vocabulario cerrado SÍ lo es, y el error nombra el
// directorio culpable — se descubre en el validador de la spec 04, antes del
// primer step, que es el único momento en que fallar no deja efectos a medias en
// la nube.
type StepConfigRepository interface {
	Get(ctx *context.Context, pipelineLocalPath, step string) (StepConfig, error)
}
