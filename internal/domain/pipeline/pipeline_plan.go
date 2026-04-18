package pipeline

import (
	"errors"
	"fmt"
)

type PipelinePlan struct {
	environment Environment
	steps       []*Step
}

func NewPipelinePlan(
	env Environment,
	steps []*Step) (*PipelinePlan, error) {

	if len(steps) == 0 {
		return nil, errors.New("el plan de ejecución debe contener al menos un paso")
	}

	// Los steps deben llegar ordenados crecientemente sin órdenes duplicados.
	// Si llegan desordenados es un bug del PlanBuilder.
	for i := 1; i < len(steps); i++ {
		prev := steps[i-1].Name().Order()
		curr := steps[i].Name().Order()
		if curr <= prev {
			return nil, fmt.Errorf(
				"los pasos deben llegar ordenados sin órdenes duplicados: paso[%d].order=%d >= paso[%d].order=%d",
				i-1, prev, i, curr,
			)
		}
	}

	return &PipelinePlan{
		environment: env,
		steps:       steps,
	}, nil
}

func (p *PipelinePlan) Environment() Environment {
	return p.environment
}

func (p *PipelinePlan) Steps() []*Step {
	out := make([]*Step, len(p.steps))
	copy(out, p.steps)
	return out
}

func (p *PipelinePlan) StepByName(name string) (*Step, bool) {
	for _, s := range p.steps {
		if s.Name().Name() == name {
			return s, true
		}
	}
	return nil, false
}
