package deployment

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// El ancla de un rollback (spec 28).
//
// # Un rollback NO es una operación de deshacer
//
// Es **una política de resolución de estado** (§5.1'), y la distinción es lo que
// hace que quepa en el modelo sin añadirle nada: lo único que cambia respecto de
// una ejecución normal es **de dónde sale el registro con el que se compara**.
// Ni la huella, ni las reglas, ni el ciclo del step se enteran.
//
// De ahí que este archivo no tenga ni una operación: son value objects
// inmutables, resueltos antes del primer step y leídos por los tres sitios que
// preguntan «¿qué registro está vigente aquí?».
//
// # Y vive en `deployment` porque el ancla nombra un DESPLIEGUE
//
// El par que el usuario escribe es `{deployment_id, attempt}`, los dos value
// objects de este paquete, y lo que valida es la lista de steps del OBJETO. Lo
// que NO vive aquí es cómo se lee —eso sale de los hechos, y `record` importa a
// `deployment` y no al revés—: el resolutor es un puerto de la cadena de
// pipeline (`pipeline.RollbackResolver`) con una implementación que escanea.

// RollbackTarget es la ejecución pasada a la que se vuelve.
//
// Es un par y no un identificador suelto porque un `deployment_id` puede tener
// más de un intento, y a lo que se ancla es a UNO: los registros que estuvieron
// vigentes son los de un intento concreto, no los del despliegue en abstracto.
//
// Las dos mitades son value objects y no cadenas y enteros a propósito (§9): un
// `deployment_id` mal formado o un `attempt: 0` fallan al componer el par, no se
// convierten en un ancla vacía que el motor confundiría con «no se pidió
// rollback».
type RollbackTarget struct {
	deployment DeploymentID
	attempt    Attempt
}

// NewRollbackTarget compone el destino.
func NewRollbackTarget(deployment DeploymentID, attempt Attempt) (RollbackTarget, error) {
	if deployment.IsZero() {
		return RollbackTarget{}, fmt.Errorf("deployment: el destino del rollback no dice a qué despliegue vuelve")
	}
	if attempt.IsZero() {
		return RollbackTarget{}, fmt.Errorf(
			"deployment: el destino del rollback no dice a qué intento vuelve (el 0 no es un intento)")
	}
	return RollbackTarget{deployment: deployment, attempt: attempt}, nil
}

// ParseRollbackTarget compone el destino desde su forma externa, que es como
// llega en el `RequestInput`.
//
// Falla EN EL BORDE, que es el punto: los dos componentes se validan con los
// mismos constructores que el resto del motor, así que un identificador mal
// escrito se rechaza como error de invocación en vez de convertirse en una
// consulta sin sujeto tres capas más adentro.
func ParseRollbackTarget(deploymentID string, attempt int) (RollbackTarget, error) {
	id, err := ParseDeploymentID(strings.TrimSpace(deploymentID))
	if err != nil {
		return RollbackTarget{}, fmt.Errorf("rollback_to: %w", err)
	}
	number, err := NewAttempt(attempt)
	if err != nil {
		return RollbackTarget{}, fmt.Errorf("rollback_to: %w", err)
	}
	return NewRollbackTarget(id, number)
}

func (t RollbackTarget) Deployment() DeploymentID { return t.deployment }
func (t RollbackTarget) Attempt() Attempt         { return t.attempt }

// IsZero indica que no se pidió ningún rollback. Es el caso normal.
func (t RollbackTarget) IsZero() bool { return t.deployment.IsZero() || t.attempt.IsZero() }

// String es la forma legible del destino, para diagnósticos.
func (t RollbackTarget) String() string {
	if t.IsZero() {
		return ""
	}
	return t.deployment.String() + " intento " + t.attempt.String()
}

// Equals es la regla de igualdad del value object.
func (t RollbackTarget) Equals(other RollbackTarget) bool {
	return t.deployment.Equals(other.deployment) && t.attempt.Equals(other.attempt)
}

// AnchoredStep es lo que el ancla sabe de UN step de la ejecución destino, y
// mezcla las dos fuentes a propósito porque hacen falta las dos:
//
//	StepID, Scope      →  del OBJETO de aquel despliegue: qué steps declaraba la
//	                      operación y bajo qué ámbito se recordaba cada uno
//	Key, RecordID      →  de los HECHOS: qué registro estuvo VIGENTE
//	                      (`evidence_from` de su `step_finished`)
//
// El objeto solo no basta —no dice qué registro se usó— y los hechos solos
// tampoco: la lista de steps que hay que cotejar con la operación de hoy es la
// que el objeto declara, no la de los que dejaron rastro.
type AnchoredStep struct {
	// StepID es el nombre del directorio del step, con su prefijo de orden.
	StepID string

	// Scope es el ámbito DECLARADO en el objeto de aquel despliegue, en su forma
	// externa (`project`, `environment`). Se coteja contra el de hoy: `01-test`
	// conserva su `step_id` a través de un cambio de ámbito y aun así su ancla
	// apunta a una clave de estado que ya no se consulta (spec 24, recuadro).
	Scope string

	// Key y RecordID son la referencia al registro vigente. **Van en cero cuando
	// aquel step no dejó ninguno**, que es lo normal para los tres casos que
	// ejecutan y no persisten —sin `config.yaml`, sin comandos, sin `rules`— y no
	// es un hueco: son steps que se ejecutan siempre, así que su comportamiento
	// en el rollback es el mismo que tuvieron entonces por construcción (§5.2).
	Key      state.Key
	RecordID state.RecordID
}

// Remembers dice que este step dejó un registro al que anclar.
func (s AnchoredStep) Remembers() bool { return !s.Key.IsZero() && !s.RecordID.IsZero() }

// RollbackAnchor es la lista inmutable `(step_id → record_id)` resuelta ANTES
// del primer step.
//
// # Que sea inmutable es la garantía, no una comodidad
//
// Los registros que el rollback escribe no cambian el ancla del rollback (§7):
// sin esa propiedad, el segundo step de R leería lo que acaba de escribir el
// primero y la vuelta atrás se contaminaría a sí misma a mitad de camino.
//
// # Y lleva el `content_id` de aquel despliegue
//
// Para poder decir si R es de verdad la misma intención que E. Lo es cuando el
// clon del pipelinecode sigue dentro de su ventana; si el pipelinecode cambió y
// el clon se refrescó, R compone otro `content_id` y **deja de ser un rollback
// en sentido estricto** (spec 18, recuadro). Eso se AVISA y no aborta, porque
// §7 exige lo contrario de abortar: un step cuyo `commands.yaml` cambió entre E
// y R **se ejecuta**, aunque su registro anclado exista. Un rollback no es un
// permiso para saltarse comprobaciones.
type RollbackAnchor struct {
	target    RollbackTarget
	contentID ContentID
	steps     []AnchoredStep
}

// NewRollbackAnchor compone el ancla.
//
// Un destino sin ni un step es un error y no un ancla vacía: significaría que el
// objeto de aquel despliegue no declara ninguno, y anclar a eso equivale a no
// anclar nada — que es exactamente el modo de fallo silencioso que §4 rechaza en
// la alternativa D.
func NewRollbackAnchor(
	target RollbackTarget, contentID ContentID, steps []AnchoredStep) (RollbackAnchor, error) {

	if target.IsZero() {
		return RollbackAnchor{}, fmt.Errorf("deployment: el ancla no dice a qué ejecución vuelve")
	}
	if contentID.IsZero() {
		return RollbackAnchor{}, fmt.Errorf(
			"deployment: el ancla de %s no dice qué contenido se pretendía desplegar", target)
	}
	if len(steps) == 0 {
		return RollbackAnchor{}, fmt.Errorf(
			"deployment: el ancla de %s no tiene ni un step al que volver", target)
	}
	return RollbackAnchor{target: target, contentID: contentID, steps: slices.Clone(steps)}, nil
}

// IsZero indica que esta ejecución no es un rollback.
func (a RollbackAnchor) IsZero() bool { return a.target.IsZero() }

func (a RollbackAnchor) Target() RollbackTarget { return a.target }
func (a RollbackAnchor) ContentID() ContentID   { return a.contentID }

// Steps son los steps anclados, en el orden en que el objeto de aquel despliegue
// los declara. Devuelve una copia: el ancla no se toca.
func (a RollbackAnchor) Steps() []AnchoredStep { return slices.Clone(a.steps) }

// AnchoredRecord es la referencia al registro que estuvo vigente para un step.
//
// La tercera salida es falsa por DOS motivos que aquí valen lo mismo: el step no
// estaba en la operación anclada, o estaba y no dejó registro. Los dos
// significan «no hay a qué anclar este step», y quien pregunta hace lo mismo con
// los dos — no encontrar registro es «ejecuta». La diferencia que sí importa
// —que la operación de hoy declare otros steps— se resuelve ANTES, en `Validate`,
// y allí es un rechazo y no una ausencia.
//
// Satisface `step.AnchorLookup`, que es el puerto mínimo que la cadena de step
// declara: `step` no puede nombrar este tipo, porque el import va de `deployment`
// a `step` y no al revés.
func (a RollbackAnchor) AnchoredRecord(stepID string) (state.Key, state.RecordID, bool) {
	for _, step := range a.steps {
		if step.StepID != stepID {
			continue
		}
		if !step.Remembers() {
			return state.Key{}, state.RecordID{}, false
		}
		return step.Key, step.RecordID, true
	}
	return state.Key{}, state.RecordID{}, false
}

// Validate coteja el ancla con la operación que se va a ejecutar.
//
// # Es la tercera condición de §5.2, y la puso al descubierto la spec 24
//
// Un intento anterior al reparto de aquélla lleva en su objeto los `step_id` de
// su generación, y `record history` lo sigue marcando como destino válido
// —porque lo es por su propio criterio, todos sus steps cerraron bien—. Pero el
// ancla es una lista `(step_id → record_id)`, así que si ninguno de esos
// identificadores existe en la operación de hoy el resultado sería **un rollback
// que no ancla ni un solo step y no lo dice**.
//
// Se cotejan las dos cosas que hacen que un ancla apunte a algo:
//
//	la LISTA de steps  —  nombres y orden
//	el ÁMBITO de cada uno  —  `01-test` conserva su nombre a través de un
//	                          cambio de ámbito y aun así su ancla apunta a una
//	                          clave de estado que ya no se consulta
//
// Los dos datos están en el objeto de despliegue, así que la comprobación no
// exige un hecho nuevo ni una migración: es comparar dos listas antes del primer
// step y rechazar nombrando las que sobran y las que faltan.
func (a RollbackAnchor) Validate(content Content) error {
	if a.IsZero() {
		return nil
	}

	steps := content.Steps()
	actuales := make([]string, 0, len(steps))
	ambitos := make(map[string]string, len(steps))
	for _, step := range steps {
		actuales = append(actuales, step.StepID())
		ambitos[step.StepID()] = step.Config().Scope().String()
	}

	anclados := make([]string, 0, len(a.steps))
	for _, step := range a.steps {
		anclados = append(anclados, step.StepID)
	}

	if faltan, sobran := diferencia(anclados, actuales); len(faltan) > 0 || len(sobran) > 0 {
		return fmt.Errorf(
			"la operación de hoy no declara los mismos steps que %s: faltan [%s], sobran [%s]"+
				" — un rollback a una lista de steps distinta no anclaría ninguno",
			a.target, strings.Join(faltan, ", "), strings.Join(sobran, ", "))
	}

	for _, step := range a.steps {
		if actual := ambitos[step.StepID]; actual != step.Scope {
			return fmt.Errorf(
				"el step '%s' se recordaba en el ámbito '%s' en %s y hoy declara '%s':"+
					" su ancla apunta a una clave de estado que ya no se consulta",
				step.StepID, vacioONoConsta(step.Scope), a.target, vacioONoConsta(actual))
		}
	}
	return nil
}

// Matches dice si la operación de hoy es la MISMA intención que la anclada.
//
// Falso no es un error (ver `RollbackAnchor`): es «esto se parece a un rollback
// y no lo es del todo», y quien lo consulta avisa. Abortar contradiría a §7, que
// exige que un step cuyo `commands.yaml` cambió se ejecute en R.
func (a RollbackAnchor) Matches(content Content) bool {
	return !a.IsZero() && a.contentID.Equals(content.ID())
}

// diferencia son los elementos de `esperados` que no están en `actuales` y al
// revés. El orden no se compara: dos listas con los mismos nombres en otro orden
// no pueden darse —el orden lo fija el prefijo `NN` del directorio— y exigirlo
// daría un diagnóstico peor.
func diferencia(esperados, actuales []string) (faltan, sobran []string) {
	presentes := make(map[string]struct{}, len(actuales))
	for _, nombre := range actuales {
		presentes[nombre] = struct{}{}
	}
	anclados := make(map[string]struct{}, len(esperados))
	for _, nombre := range esperados {
		anclados[nombre] = struct{}{}
		if _, ok := presentes[nombre]; !ok {
			faltan = append(faltan, nombre)
		}
	}
	for _, nombre := range actuales {
		if _, ok := anclados[nombre]; !ok {
			sobran = append(sobran, nombre)
		}
	}
	return faltan, sobran
}

func vacioONoConsta(valor string) string {
	if valor == "" {
		return "(no consta)"
	}
	return valor
}
