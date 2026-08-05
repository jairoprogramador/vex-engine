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

// Evaluate compara la huella de contenido del proyecto y NO escribe: devuelve lo
// observado para que `StepExecutable` lo persista tras el éxito (spec 09 §5.1).
//
// El valor que compara y transporta es la forma canónica de la huella v1 —con su
// prefijo `v1:`—, tal cual la calcula el handler 08 del pipeline. La evidencia no
// lo recorta: una huella sin versión es indistinguible de una de otra versión
// (spec 08 §5.3).
func (s CodeProjectRuleRule) Evaluate(ctx RuleContext) (Decision, []Evidence, error) {
	fingerprintCurrent, err := GetParam[string](ctx, ProjectStatusCurrentParam)
	if err != nil {
		return s.sinObservar("no se pudo obtener el estado actual del proyecto", err)
	}

	projectUrl, err := GetParam[string](ctx, ProjectUrlParam)
	if err != nil {
		return s.sinObservar("no se pudo obtener la url del proyecto", err)
	}

	pipelineUrl, err := GetParam[string](ctx, PipelineUrlParam)
	if err != nil {
		return s.sinObservar("no se pudo obtener la url del pipeline", err)
	}

	step, err := GetParam[string](ctx, StepParam)
	if err != nil {
		return s.sinObservar("no se pudo obtener el paso de ejecución", err)
	}

	fingerprintPrevious, err := s.repository.Get(projectUrl, pipelineUrl, step)
	if err != nil {
		return DecisionUndetermined("no se pudo leer el estado anterior del proyecto"),
			[]Evidence{NewEvidence(CodeProjectRuleName, fingerprintCurrent, "", false)},
			err
	}

	evidence := []Evidence{NewEvidence(
		CodeProjectRuleName,
		fingerprintCurrent,
		fingerprintPrevious,
		fingerprintCurrent != fingerprintPrevious,
	)}

	if fingerprintCurrent == fingerprintPrevious {
		return DecisionSkip("el código del proyecto no ha cambiado"), evidence, nil
	}
	return DecisionRun("el código del proyecto ha cambiado"), evidence, nil
}

func (s CodeProjectRuleRule) sinObservar(reason string, err error) (Decision, []Evidence, error) {
	return DecisionUndetermined(reason), []Evidence{NoEvidence(CodeProjectRuleName)}, err
}
