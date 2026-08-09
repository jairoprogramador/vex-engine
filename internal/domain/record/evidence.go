package record

import (
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// EvidenceRef es la procedencia del registro que un `step_finished` invoca:
// **qué registro estuvo vigente para este step**, y de qué despliegue salió.
//
// # Cierra I-5, que es la afirmación de valor del motor
//
// Con `from_cache: true` y una huella opaca, «¿cuándo se testeó esto por última
// vez?» no tendría respuesta — y esa pregunta ES lo que el motor promete. La
// procedencia ya se guarda desde la spec 10 —hoy en `state.Provenance`, dentro
// del registro (spec 11)—; aquí se emite, no se recalcula.
//
// # Y lleva la mitad simétrica que la spec 28 necesita
//
// La referencia al registro viaja **también cuando el step SE EJECUTÓ** —el que
// acaba de escribir—, no sólo cuando revivió. Sin esa mitad, un rollback anclado
// a una ejecución pasada tendría que derivar por fechas qué registro estuvo
// vigente, que es guardar una conclusión en vez de un hecho (28 §5.3).
//
// # Los campos de despliegue pueden ir vacíos, y es una verdad, no un hueco
//
// `state.Provenance` es hoy `{ExecutionID, At}`: no lleva `deployment_id` ni
// `attempt` todavía. Un registro escrito antes de que los lleve produce una
// evidencia sin ellos, y eso es exactamente lo que se sabe de él. Lo que NO
// puede faltar es la referencia al registro: sin ella la evidencia no apunta a
// nada.
type EvidenceRef struct {
	// ExecutionID es la ejecución que dejó el registro. Es lo que
	// `state.Provenance` sabe hoy, y por eso es obligatorio.
	ExecutionID string

	// Deployment y Attempt son la posición de aquel despliegue en la historia de
	// su ambiente. Se pueblan cuando `state.Provenance` los lleve; hasta
	// entonces van en cero.
	Deployment deployment.DeploymentID
	Attempt    deployment.Attempt

	// At es cuándo se produjo el registro invocado. Es la respuesta literal a
	// «¿cuándo se testeó esto por última vez?».
	At time.Time

	// StateKey y RecordID son la referencia: dónde vive el registro y cuál de
	// ellos es. Es el mismo par que el índice del caché guarda —`{state_key,
	// record_id}`—, así que el dato ya existe y no hay que derivarlo.
	StateKey state.Key
	RecordID state.RecordID
}

// IsZero dice que el hecho no invoca ningún registro. Es lo normal en un step
// que se ejecutó sin poder recordarse: sin `config.yaml`, sin comandos o sin
// `rules`, los tres ejecutan, terminan bien y no escriben registro.
func (r EvidenceRef) IsZero() bool {
	return r.ExecutionID == "" && r.RecordID.IsZero() && r.StateKey.IsZero() && r.At.IsZero()
}

// Validate exige que una evidencia presente esté COMPLETA en lo que la hace
// utilizable. Una evidencia a medias es peor que ninguna: afirma que hay un
// registro detrás y no deja llegar hasta él.
func (r EvidenceRef) Validate() error {
	if r.IsZero() {
		return nil
	}
	if r.ExecutionID == "" {
		return fmt.Errorf("record: la evidencia no dice qué ejecución produjo el registro")
	}
	if r.At.IsZero() {
		return fmt.Errorf("record: la evidencia no dice cuándo se produjo el registro")
	}
	if r.StateKey.IsZero() {
		return fmt.Errorf("record: la evidencia no dice dónde vive el registro")
	}
	if r.RecordID.IsZero() {
		return fmt.Errorf("record: la evidencia no dice qué registro es")
	}
	return nil
}
