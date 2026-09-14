package command

type StepStatus string

const (
	StepSuccess    StepStatus = "SUCCESS"
	StepFailure    StepStatus = "FAILURE"
	StepCached     StepStatus = "CACHED"
	StepRegistered StepStatus = "REGISTERED"
	StepRunning    StepStatus = "RUNNING"

	// StepSkipped es el step que no se ejecutó y no por caché: hoy solo el que
	// no tiene comandos. No es SUCCESS porque SUCCESS afirma que algo se
	// ejecutó correctamente, y sobre cero comandos eso es una conclusión, no un
	// hecho (spec 04 §5.3, D-A12).
	StepSkipped StepStatus = "SKIPPED"
)

func (s StepStatus) String() string {
	return string(s)
}

func (s StepStatus) IsTerminal() bool {
	return s == StepSuccess || s == StepFailure || s == StepCached || s == StepSkipped
}
