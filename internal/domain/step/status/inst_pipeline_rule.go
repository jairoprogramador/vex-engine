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

// Evaluate compara la huella de las instrucciones y NO escribe: devuelve lo
// observado para que `StepExecutable` lo persista tras el éxito (spec 09 §5.1).
func (r InstructionsPipelineRule) Evaluate(ctx RuleContext) (Decision, []Evidence, error) {
	commands, err := GetParam[[]command.Command](ctx, InstCurrentParam)
	if err != nil {
		return r.sinObservar("no se pudo obtener el estado actual de las instrucciones", err)
	}

	step, err := GetParam[string](ctx, StepParam)
	if err != nil {
		return r.sinObservar("no se pudo obtener el paso de ejecución", err)
	}

	projectUrl, err := GetParam[string](ctx, ProjectUrlParam)
	if err != nil {
		return r.sinObservar("no se pudo obtener la url del proyecto", err)
	}

	pipelineUrl, err := GetParam[string](ctx, PipelineUrlParam)
	if err != nil {
		return r.sinObservar("no se pudo obtener la url del pipeline", err)
	}

	instCurrentFingerprint, err := r.calculateFingerprint(commands)
	if err != nil {
		return r.sinObservar("no se pudo calcular el estado actual de las instrucciones", err)
	}

	instPreviousFingerprint, err := r.repository.Get(projectUrl, pipelineUrl, step)
	if err != nil {
		// La huella actual SÍ se observó: viaja en la evidencia para que, si el
		// step termina bien, quede escrita. Lo que no se pudo leer es la
		// anterior, y por eso la decisión es «no se sabe», no «cambió».
		return DecisionUndetermined("no se pudo leer el estado anterior de las instrucciones"),
			[]Evidence{NewEvidence(InstPipelineRuleName, instCurrentFingerprint, "", false)},
			err
	}

	evidence := []Evidence{NewEvidence(
		InstPipelineRuleName,
		instCurrentFingerprint,
		instPreviousFingerprint,
		instCurrentFingerprint != instPreviousFingerprint,
	)}

	if instCurrentFingerprint == instPreviousFingerprint {
		return DecisionSkip("las instrucciones del pipeline no han cambiado"), evidence, nil
	}
	return DecisionRun("las instrucciones del pipeline han cambiado"), evidence, nil
}

func (r InstructionsPipelineRule) sinObservar(reason string, err error) (Decision, []Evidence, error) {
	return DecisionUndetermined(reason), []Evidence{NoEvidence(InstPipelineRuleName)}, err
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
