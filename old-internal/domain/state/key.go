package state

import (
	"fmt"
	"strings"
)

// Key es la clave de POSICIÓN del estado de un step: dónde se recuerda lo que
// ese step dejó.
//
//	clave_estado = (subject, scope, step_id)
//
// Su composición ES la tesis de la spec 11, y lo que NO entra pesa tanto como
// lo que entra:
//
//   - `subject` es el identificador del proyecto —el mismo del objeto de
//     despliegue, spec 17—. Garantiza que dos proyectos que compartan plantilla
//     no colisionen.
//   - `scope` es dirección, no contenido: ver Scope.
//   - `step_id` es el nombre del DIRECTORIO del step, CON su prefijo de orden
//     (`02-provision`). Ver el aviso de abajo.
//   - **el pipeline NO entra** (D-A14, cerrada al revés). El modo de fallo que
//     motivaba meterlo —dos pipelines que comparten estado y leen un valor ajeno
//     en silencio— lo cierra la huella: revivir exige igualdad de
//     `step_fingerprint`, que incluye el pipelinecode entero, así que dos
//     pipelines distintos nunca reviven el registro del otro aunque compartan
//     clave. Cerrado el modo de fallo, manda el argumento conceptual: **el ACR
//     pertenece al proyecto, no al pipeline**, y un proyecto que cambia de
//     plantilla no debe perder de vista los recursos que ya creó.
//
// Residuo declarado, no resuelto (spec 11 §5.2): los registros de dos pipelines
// se intercalan bajo la misma clave, así que una regla de expiración por edad
// mide contra el último registro, que puede ser de otro pipeline.
//
// > **La identidad del step es su ruta, y eso tiene un coste.** `step_id` lleva
// > el prefijo de orden, así que **renumerar un step pierde su historia**:
// > `02-supply` → `03-supply` es, para el almacén, un step que nunca corrió. Es
// > consecuencia directa de que el diseño identifique un step por su ruta, y es
// > el precio de no inventar un identificador estable que el autor tendría que
// > mantener a mano. El efecto es una re-ejecución, nunca un despliegue omitido.
type Key struct {
	subject string
	scope   Scope
	stepID  string
}

// NewKey compone la clave. Un componente ausente es un ERROR y no una clave
// degradada, por la misma razón que en `cache.Material`: una clave con un hueco
// es perfectamente válida y colisiona con la de cualquier otra a la que le falte
// lo mismo — y aquí esa colisión se manifestaría como un step que lee el estado
// de otro.
func NewKey(subject string, scope Scope, stepID string) (Key, error) {
	if subject == "" {
		return Key{}, fmt.Errorf("state: la clave no tiene subject")
	}
	if scope.IsZero() {
		return Key{}, fmt.Errorf("state: la clave no tiene ámbito")
	}
	if stepID == "" {
		return Key{}, fmt.Errorf("state: la clave no tiene step")
	}
	if strings.ContainsAny(stepID, `/\`) || stepID == "." || stepID == ".." {
		return Key{}, fmt.Errorf("state: %q no puede usarse como step_id", stepID)
	}
	return Key{subject: subject, scope: scope, stepID: stepID}, nil
}

func (k Key) Subject() string { return k.subject }
func (k Key) Scope() Scope    { return k.scope }
func (k Key) StepID() string  { return k.stepID }

// IsZero indica que no hay clave.
func (k Key) IsZero() bool {
	return k.subject == "" || k.scope.IsZero() || k.stepID == ""
}

// Equals es la regla de igualdad del value object.
func (k Key) Equals(other Key) bool {
	return k.subject == other.subject &&
		k.scope.Equals(other.scope) &&
		k.stepID == other.stepID
}

// String es una forma legible para diagnósticos y mensajes. NO es un formato de
// serialización: el índice guarda los tres componentes por separado, para no
// tener que inventar un escape del separador.
func (k Key) String() string {
	if k.IsZero() {
		return ""
	}
	return k.subject + " " + k.scope.String() + " " + k.stepID
}
