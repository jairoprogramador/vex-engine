package status

const (
	CodeProjectRuleName       = "code_project_rule"
	ProjectStatusCurrentParam = "project_status_current"
)

type CodeProjectRuleRule struct {
	repository CodeStatusRepository
}

func NewCodeProjectRuleRule(repository CodeStatusRepository) CodeProjectRuleRule {
	return CodeProjectRuleRule{repository: repository}
}

func (s CodeProjectRuleRule) Name() string { return CodeProjectRuleName }

func (s CodeProjectRuleRule) Evaluate(ctx RuleContext) (Decision, error) {
	fingerprintCurrent, err := GetParam[string](ctx, ProjectStatusCurrentParam)
	if err != nil {
		return DecisionRun("error al obtener el estado actual del proyecto"), err
	}

	projectUrl, err := GetParam[string](ctx, ProjectUrlParam)
	if err != nil {
		return DecisionRun("error al obtener la url del projecto"), err
	}

	fingerprintPrevious, err := s.repository.Get(projectUrl)
	if err != nil {
		return DecisionRun("error al obtener el estado anterior del proyecto"), err
	}

	if fingerprintCurrent == fingerprintPrevious {
		return DecisionSkip("el código del proyecto no ha cambiado"), nil
	} else {
		err = s.repository.Set(projectUrl, fingerprintCurrent)
		if err != nil {
			return DecisionRun("no se ha podido guardar el estado del proyecto"), err
		}
	}

	return DecisionRun("el codigo del proyecto a cambiado"), nil
}
