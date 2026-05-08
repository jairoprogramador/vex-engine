package status

import (
	"time"
)

const (
	TimeRuleName     = "time_rule"
	CurrentTimeParam = "current_time"
)

const defaultTTLDuration = 30 * 24 * time.Hour

type TimeRule struct {
	repository TimeStatusRepository
}

func NewTimeRule(repository TimeStatusRepository) TimeRule {
	return TimeRule{repository: repository}
}

func (e TimeRule) Name() string { return TimeRuleName }

func (e TimeRule) Evaluate(ctx RuleContext) (Decision, error) {
	currentTime, err := GetParam[time.Time](ctx, CurrentTimeParam)
	if err != nil {
		return DecisionRun("error al obtener el tiempo actual"), err
	}

	projectUrl, err := GetParam[string](ctx, ProjectUrlParam)
	if err != nil {
		return DecisionRun("error al obtener la url del projecto"), err
	}

	environment, err := GetParam[string](ctx, EnvironmentParam)
	if err != nil {
		return DecisionRun("error al obtener el ambiente de ejecucion"), err
	}

	step, err := GetParam[string](ctx, StepParam)
	if err != nil {
		return DecisionRun("error al obtener el paso de ejecucion"), err
	}

	previousTime, err := e.repository.Get(projectUrl, environment, step)
	if err != nil {
		return DecisionRun("error al obtener el time de la ejecucion anterior"), err
	}

	expiration := previousTime.Add(defaultTTLDuration)
	if currentTime.Before(expiration) {
		return DecisionSkip("el tiempo a expirado, se vuelve a ejecutar"), nil
	} else {
		err = e.repository.Set(projectUrl, environment, step, currentTime)
		if err != nil {
			return DecisionRun("no se ha podido guardar el estado del tiempo de ejecucion"), err
		}
	}

	return DecisionRun("el tiempo a expirado, se vuelve a ejecutar"), nil
}
