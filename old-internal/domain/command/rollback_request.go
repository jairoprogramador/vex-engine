package command

// RollbackRequest es el destino de rollback tal como el `RequestInput` lo
// nombra, SIN interpretar (spec 28 §5.5).
//
// # Por qué viaja como dato crudo y no como value object
//
// Porque tiene que atravesar `command`, y `command` no puede nombrar
// `deployment.RollbackTarget`: el import va de `deployment` a `step` y de `step`
// a `command`, así que la vuelta cerraría el ciclo. Es el mismo reparto que
// obliga a `step.FactSink` a existir aparte de `record.Facts`.
//
// La validación NO se pierde por ello, y ocurre dos veces a propósito:
//
//	en el BORDE   `vexd run` compone `deployment.ParseRollbackTarget` antes de
//	              ejecutar nada, así que un `deployment_id` mal formado o un
//	              `attempt: 0` salen con exit code 2 —error de invocación— y no
//	              como un fallo de pipeline a mitad;
//	en el DOMINIO el handler 09 vuelve a componerlo para tener el tipo en la
//	              mano. Ahí ya no puede fallar; que el constructor sea el mismo
//	              es lo que garantiza que no haya dos definiciones de «bien
//	              formado».
//
// El valor cero significa «esto no es un rollback», que es el caso normal.
type RollbackRequest struct {
	DeploymentID string
	Attempt      int
}

// IsZero indica que no se pidió ningún rollback.
func (r RollbackRequest) IsZero() bool { return r.DeploymentID == "" && r.Attempt == 0 }
