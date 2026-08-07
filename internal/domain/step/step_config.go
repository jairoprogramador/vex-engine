package step

import (
	"context"
	"fmt"
)

// StepConfig es lo que un step declara SOBRE SÍ MISMO en
// `steps/NN-nombre/config.yaml`.
//
// Hoy sólo lleva el ámbito. La spec 15 añade aquí las reglas de re-ejecución, y
// van juntas en el archivo y separadas como conceptos a propósito: el ámbito es
// DÓNDE vive el estado, la regla es CUÁNDO ese estado deja de ser válido. Son
// ortogonales, y mezclarlas haría más difícil razonar sobre cada una.
//
// «Un step, un ámbito» es una invariante de AGREGADO y no una convención de
// estilo: es lo que permite que un step tenga una identidad y una huella. Un
// agregado con dos criterios de cambio no es un agregado — por eso el tipo lleva
// un `Scope` y no una lista, y por eso un step que necesita los dos se parte en
// dos steps, que el orden numérico ya secuencia (spec 13 §5.2).
type StepConfig struct {
	declared bool
	scope    Scope
}

// NewStepConfig es la configuración de un step que SÍ declara. El ámbito vacío
// no puede llegar aquí: quien lo construye ya pasó por `NewScope`.
func NewStepConfig(scope Scope) (StepConfig, error) {
	if scope.IsZero() {
		return StepConfig{}, fmt.Errorf("step: la configuración no declara ámbito")
	}
	return StepConfig{declared: true, scope: scope}, nil
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
