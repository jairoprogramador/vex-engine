package command

type Step struct {
	name StepName
}

func NewStep(name StepName) Step {
	return Step{
		name: name,
	}
}

func (s Step) Name() string {
	return s.name.Name()
}

func (s Step) FullName() string {
	return s.name.FullName()
}
