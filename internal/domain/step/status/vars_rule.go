package status

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

const (
	VariablesRuleName     = "variables_rule"
	VariablesCurrentParam = "variables_current"
)

const varsFingerprintFieldSep = '\x1e'

type VariablesRuleRule struct {
	repository VariablesStatusRepository
}

func NewVariablesRuleRule(repository VariablesStatusRepository) VariablesRuleRule {
	return VariablesRuleRule{repository: repository}
}

func (s VariablesRuleRule) Name() string { return VariablesRuleName }

// Evaluate compara la huella de las variables y NO escribe: devuelve lo
// observado para que `StepExecutable` lo persista tras el éxito (spec 09 §5.1).
func (s VariablesRuleRule) Evaluate(ctx RuleContext) (Decision, []Evidence, error) {
	variablesCurrentOriginal, err := GetParam[*command.ExecutionVariableMap](ctx, VariablesCurrentParam)
	if err != nil {
		return s.sinObservar("no se pudo obtener el estado actual de las variables", err)
	}

	projectUrl, err := GetParam[string](ctx, ProjectUrlParam)
	if err != nil {
		return s.sinObservar("no se pudo obtener la url del proyecto", err)
	}

	pipelineUrl, err := GetParam[string](ctx, PipelineUrlParam)
	if err != nil {
		return s.sinObservar("no se pudo obtener la url del pipeline", err)
	}

	environment, err := GetParam[string](ctx, EnvironmentParam)
	if err != nil {
		return s.sinObservar("no se pudo obtener el ambiente de ejecución", err)
	}

	step, err := GetParam[string](ctx, StepParam)
	if err != nil {
		return s.sinObservar("no se pudo obtener el paso de ejecución", err)
	}

	variablesClone := variablesCurrentOriginal.Clone()
	variablesClone.Remove(command.VarProjectVersion)
	variablesClone.Remove(command.VarProjectRevision)
	variablesClone.Remove(command.VarProjectRevisionFull)
	variablesClone.Remove(command.VarToolName)
	variablesClone.Remove(command.VarProjectWorkdir)
	variablesClone.Remove(command.VarStepWorkdir)

	varsCurrentFingerprint, err := s.calculateFingerprint(variablesClone)
	if err != nil {
		return s.sinObservar("no se pudo calcular el estado actual de las variables", err)
	}

	varsPreviousFingerprint, err := s.repository.Get(projectUrl, pipelineUrl, environment, step)
	if err != nil {
		return DecisionUndetermined("no se pudo leer el estado anterior de las variables"),
			[]Evidence{NewEvidence(VariablesRuleName, varsCurrentFingerprint, "", false)},
			err
	}

	evidence := []Evidence{NewEvidence(
		VariablesRuleName,
		varsCurrentFingerprint,
		varsPreviousFingerprint,
		varsCurrentFingerprint != varsPreviousFingerprint,
	)}

	if varsCurrentFingerprint == varsPreviousFingerprint {
		return DecisionSkip("las variables no han cambiado"), evidence, nil
	}
	return DecisionRun("las variables han cambiado"), evidence, nil
}

func (s VariablesRuleRule) sinObservar(reason string, err error) (Decision, []Evidence, error) {
	return DecisionUndetermined(reason), []Evidence{NoEvidence(VariablesRuleName)}, err
}

func (s VariablesRuleRule) canonicalVariableMaterial(variable command.Variable) string {
	var b strings.Builder
	b.WriteString(strconv.Quote(variable.Name()))
	b.WriteByte(varsFingerprintFieldSep)
	b.WriteString(strconv.Quote(variable.Value()))
	b.WriteByte(varsFingerprintFieldSep)
	b.WriteString(strconv.FormatBool(variable.IsShared()))
	return b.String()
}

func (s VariablesRuleRule) canonicalVariablesMapMaterial(variablesMap command.ExecutionVariableMap) string {
	slice := variablesMap.ToSlice()
	slices.SortFunc(slice, func(a, b command.Variable) int {
		return strings.Compare(a.Name(), b.Name())
	})
	var builder strings.Builder
	for index, variable := range slice {
		if index > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(s.canonicalVariableMaterial(variable))
	}
	return builder.String()
}

func (s VariablesRuleRule) calculateFingerprint(variablesMap command.ExecutionVariableMap) (string, error) {
	variablesCurrent := variablesMap.Clone()
	material := s.canonicalVariablesMapMaterial(variablesCurrent)
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:]), nil
}
