package state

import (
	"fmt"
	"slices"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

// Provenance es qué ejecución dejó un registro, y cuándo.
//
// **Existe y no participa de ninguna decisión.** Sirve para responder «¿qué
// ejecución dejó este valor exactamente?» —trazabilidad hacia el registro de
// despliegue (spec 17) y ancla del rollback (spec 28)—. La decisión de revivir o
// ejecutar NUNCA la consulta: sólo compara `step_fingerprint`.
//
// La spec 17 la amplía con `deployment_id` y `attempt`; aquí basta con que el
// hecho se guarde en vez de perderse.
type Provenance struct {
	ExecutionID string
	At          time.Time
}

// StepRecord es lo que una ejecución real de un step dejó: un HECHO FECHADO.
//
// Es un agregado INMUTABLE: se crea completo y no tiene setters. La
// inmutabilidad no es rigor por gusto, es la implementación del append-only —si
// el objeto no se puede modificar, la tienda no puede sobrescribir—.
//
// Se escribe uno por ejecución real, **nunca cuando el step revive**: revivir no
// es un hecho nuevo del step, es la constatación de uno viejo.
type StepRecord struct {
	id              RecordID
	stepFingerprint string
	variables       []command.Variable
	producedBy      Provenance
}

// NewStepRecord compone el registro que deja un step que acaba de terminar bien.
//
// La huella PUEDE ir vacía, y es una decisión, no un descuido: cuando el motor
// no consigue componerla —material incompleto, huella del proyecto corrupta— el
// step se ejecuta igual y lo que produjo sigue siendo estado real. Guardarlo sin
// huella es exactamente la asimetría de la spec 11 §4: un registro sin huella no
// revive jamás (ver Revives), y perder de vista un ARN es peor que re-ejecutar.
//
// La huella es siempre un hash; el valor de una variable puede ir en claro. Son
// dos objetos con reglas distintas y conviene no unificarlas: la huella se
// mantiene minimalista porque puede terminar sincronizada a un backend remoto
// (spec 21), mientras que conservar el valor es lo que permite a un diagnóstico
// decir «se re-ejecuta porque DB_POOL_SIZE cambió de 10 a 50» (spec 22). El
// enmascarado de lo que sale de la organización es de la spec 20.
func NewStepRecord(
	id RecordID,
	stepFingerprint string,
	variables []command.Variable,
	producedBy Provenance) (StepRecord, error) {

	if id.IsZero() {
		return StepRecord{}, fmt.Errorf("state: un registro sin record_id no es ordenable")
	}
	if producedBy.ExecutionID == "" {
		return StepRecord{}, fmt.Errorf("state: un registro sin ejecución que lo produjo no es trazable")
	}
	if producedBy.At.IsZero() {
		return StepRecord{}, fmt.Errorf("state: un registro sin instante de producción no es trazable")
	}

	return StepRecord{
		id:              id,
		stepFingerprint: stepFingerprint,
		variables:       slices.Clone(variables),
		producedBy:      producedBy,
	}, nil
}

// NewUnattributedRecord es el registro que devuelve un almacén que guarda el
// ÚLTIMO VALOR y no historia — hoy sólo el adaptador de Supabase del modo
// remoto, que se retira en la spec 16.
//
// No tiene identificador, ni huella, ni procedencia, y eso no es una carencia
// que haya que rellenar: es la verdad sobre lo que ese almacén sabe. Las
// consecuencias son exactamente las correctas — no revive nunca (no hay huella
// que comparar), no puede anclar un rollback (no hay procedencia) y no entra en
// el índice (no hay identificador al que apuntar).
func NewUnattributedRecord(variables []command.Variable) StepRecord {
	return StepRecord{variables: slices.Clone(variables)}
}

func (r StepRecord) ID() RecordID            { return r.id }
func (r StepRecord) StepFingerprint() string { return r.stepFingerprint }
func (r StepRecord) ProducedBy() Provenance  { return r.producedBy }
func (r StepRecord) Variables() []command.Variable {
	return slices.Clone(r.variables)
}

// Revives responde la única pregunta que decide si el step vuelve a ejecutarse:
// ¿este registro corresponde EXACTAMENTE al trabajo que se iba a hacer?
//
// Una huella vacía —de cualquiera de los dos lados— nunca revive. Es lo que
// conserva por construcción la semántica «sin evidencia ⇒ ejecutar» de la
// spec 05 §5.1: un registro que no dice de qué contenido es no puede afirmar
// que ese contenido esté al día.
func (r StepRecord) Revives(stepFingerprint string) bool {
	return stepFingerprint != "" && r.stepFingerprint == stepFingerprint
}
