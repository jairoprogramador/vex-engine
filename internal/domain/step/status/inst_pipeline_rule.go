package status

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

const (
	InstPipelineRuleName = "instructions_pipeline"
	InstCurrentParam     = "instructions_current"
)

const instFingerprintFieldSep = '\x1e'

type InstructionsPipelineRule struct {
	repository InstructionsStatusRepository
}

func NewInstructionsPipelineRule(
	repository InstructionsStatusRepository) InstructionsPipelineRule {
	return InstructionsPipelineRule{repository: repository}
}

func (r InstructionsPipelineRule) Name() string { return InstPipelineRuleName }

func (r InstructionsPipelineRule) Evaluate(ctx RuleContext) (Decision, error) {
	commands, err := GetParam[[]command.Command](ctx, InstCurrentParam)
	if err != nil {
		return DecisionRun("error al obtener el estado actual de las instructiones"), err
	}

	step, err := GetParam[string](ctx, StepParam)
	if err != nil {
		return DecisionRun("error al obtener el paso de ejecucion"), err
	}

	projectUrl, err := GetParam[string](ctx, ProjectUrlParam)
	if err != nil {
		return DecisionRun("error al obtener la url del projecto"), err
	}

	pipelineUrl, err := GetParam[string](ctx, PipelineUrlParam)
	if err != nil {
		return DecisionRun("error al obtener la url del pipeline"), err
	}

	instCurrentFingerprint, err := r.calculateFingerprint(commands)
	if err != nil {
		return DecisionRun("error al calcular el estado actual de las instructions"), err
	}

	instPreviousFingerprint, err := r.repository.Get(projectUrl, pipelineUrl, step)
	if err != nil {
		return DecisionRun("error al obtener el estado anterior de las instructions"), err
	}

	if instCurrentFingerprint == instPreviousFingerprint {
		return DecisionSkip("las instrucciones del pipeline no ha cambiado"), nil
	} else {
		err = r.repository.Set(projectUrl, pipelineUrl, step, instCurrentFingerprint)
		if err != nil {
			return DecisionRun("no se ha podido guardar el estado de las instrucciones"), err
		}
	}

	return DecisionRun("las instrucciones an cambiado"), nil
}

func (r InstructionsPipelineRule) canonicalCommandMaterial(c command.Command) string {
	var b strings.Builder

	b.WriteString(strconv.Quote(c.Name()))
	b.WriteByte(instFingerprintFieldSep)
	b.WriteString(strconv.Quote(c.Cmd()))
	b.WriteByte(instFingerprintFieldSep)
	b.WriteString(strconv.Quote(filepath.ToSlash(c.Workdir().String())))

	templates := c.TemplatePaths()
	b.WriteByte(instFingerprintFieldSep)
	b.WriteString(strconv.Itoa(len(templates)))
	for _, tp := range templates {
		b.WriteByte(instFingerprintFieldSep)
		b.WriteString(strconv.Quote(filepath.ToSlash(tp.String())))
	}
	outputs := c.Outputs()
	b.WriteByte(instFingerprintFieldSep)
	b.WriteString(strconv.Itoa(len(outputs)))
	for _, op := range outputs {
		b.WriteByte(instFingerprintFieldSep)
		b.WriteString(strconv.Quote(op.Name()))
		b.WriteByte(instFingerprintFieldSep)
		b.WriteString(strconv.Quote(op.Probe()))
	}
	return b.String()
}

func (r InstructionsPipelineRule) canonicalInstructionsMaterial(commands []command.Command) string {
	var b strings.Builder
	for i, c := range commands {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(r.canonicalCommandMaterial(c))
	}
	return b.String()
}

func (r InstructionsPipelineRule) calculateFingerprint(commands []command.Command) (string, error) {
	material := r.canonicalInstructionsMaterial(commands)
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:]), nil
}
