package pipeline

type StepLimit struct {
	name string
}

// NewStepLimit construye un StepLimit. Si name es vacío, representa "todos los steps".
func NewStepLimit(name string) StepLimit {
	return StepLimit{name: name}
}

func (s StepLimit) IsAll() bool {
	return s.name == ""
}

func (s StepLimit) Name() string {
	return s.name
}
