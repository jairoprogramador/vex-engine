package status

type Decision struct {
	action Action
	reason string
}

func DecisionRun(reason string) Decision {
	return Decision{action: ActionRun, reason: reason}
}

func DecisionSkip(reason string) Decision {
	return Decision{
		action: ActionSkip,
		reason: reason,
	}
}

// DecisionUndetermined es la respuesta de una regla que no pudo averiguar si
// algo cambió: el repositorio no contestó, faltó un parámetro, la huella no se
// pudo calcular (spec 09 §5.3).
//
// Se conserva el fail-open —ejecuta, igual que `Run`— porque ejecutar de más
// nunca produce un despliegue que no ocurrió. Lo que cambia es que deja de ser
// silencioso: hasta la spec 09 un fallo de I/O devolvía `DecisionRun("error...")`
// y era indistinguible de «el código cambió».
func DecisionUndetermined(reason string) Decision {
	return Decision{action: ActionUndetermined, reason: reason}
}

// ShouldRun es fail-open: ejecuta salvo que se SEPA que no hace falta. Solo
// `Skip` —«sé que no cambió»— evita la ejecución.
func (d Decision) ShouldRun() bool { return d.action != ActionSkip }

// IsUndetermined distingue el «se ejecuta porque cambió» del «se ejecuta porque
// no se pudo averiguar». Los dos ejecutan; solo uno es una razón de negocio.
func (d Decision) IsUndetermined() bool { return d.action == ActionUndetermined }

func (d Decision) Action() Action { return d.action }
func (d Decision) Reason() string { return d.reason }
