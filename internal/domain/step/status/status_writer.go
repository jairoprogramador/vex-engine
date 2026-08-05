package status

import (
	"errors"
	"fmt"
)

// StatusWriter es el ÚNICO sitio del motor que escribe estado de re-ejecución.
//
// Antes escribían las cuatro reglas, cada una dentro de su propio `Evaluate` y
// antes de que el step ejecutara un solo comando. Concentrar la escritura aquí
// —y llamarla solo desde el camino de éxito de `StepExecutable`— es lo que cierra
// la ventana de la spec 09 §1(a): entre «se decide que hay que ejecutar» y «se
// ejecutó» ya no hay nada escrito, así que una muerte dura a mitad no puede dejar
// grabado «sin cambios» para un step que nunca terminó. La ventana se cierra por
// construcción, no por cuidado del programador, y con ella desaparece el borrado
// compensatorio que intentaba parchearla.
//
// El `switch` por nombre de regla es el precio de que las cuatro claves sean
// distintas —`time_rule` es la única sin pipeline—. La spec 10 unifica las cuatro
// entradas en una comparación de `cache_key` y este tipo se reduce a una escritura.
type StatusWriter struct {
	inst InstructionsStatusRepository
	vars VariablesStatusRepository
	code CodeStatusRepository
	time TimeStatusRepository
}

func NewStatusWriter(
	inst InstructionsStatusRepository,
	vars VariablesStatusRepository,
	code CodeStatusRepository,
	time TimeStatusRepository) *StatusWriter {

	return &StatusWriter{
		inst: inst,
		vars: vars,
		code: code,
		time: time,
	}
}

// Write persiste todo lo que las reglas observaron. Se llama una sola vez por
// step y después de que su último comando terminó bien.
//
// Escribe TODAS las evidencias, también las de las reglas que dijeron «no
// cambió»: reescribir el mismo valor no cuesta nada y es lo que hace que la
// marca de `time_rule` se refresque cuando el step se ejecuta, aunque quien lo
// mandara ejecutar fuera otra regla (spec 09 §5.4).
//
// El fallo de una escritura no aborta las demás: cada entrada es independiente y
// perder una no es motivo para perder las otras tres.
func (w *StatusWriter) Write(ctx RuleContext, evidences []Evidence) error {
	var errs []error
	for _, evidence := range evidences {
		if !evidence.Persistable() {
			// La regla no llegó a observar nada. Escribir aquí grabaría el vacío
			// como si fuera la huella del step, y la corrida siguiente lo
			// saltaría por una igualdad que nadie observó.
			continue
		}
		if err := w.write(ctx, evidence); err != nil {
			errs = append(errs, fmt.Errorf("guardar el estado de %s: %w", evidence.RuleName, err))
		}
	}
	return errors.Join(errs...)
}

func (w *StatusWriter) write(ctx RuleContext, evidence Evidence) error {
	switch evidence.RuleName {
	case InstPipelineRuleName:
		projectUrl, pipelineUrl, step, err := projectPipelineStepKey(ctx)
		if err != nil {
			return err
		}
		return w.inst.Set(projectUrl, pipelineUrl, step, evidence.Current)

	case CodeProjectRuleName:
		projectUrl, pipelineUrl, step, err := projectPipelineStepKey(ctx)
		if err != nil {
			return err
		}
		return w.code.Set(projectUrl, pipelineUrl, step, evidence.Current)

	case VariablesRuleName:
		projectUrl, pipelineUrl, step, err := projectPipelineStepKey(ctx)
		if err != nil {
			return err
		}
		environment, err := GetParam[string](ctx, EnvironmentParam)
		if err != nil {
			return err
		}
		return w.vars.Set(projectUrl, pipelineUrl, environment, step, evidence.Current)

	case TimeRuleName:
		projectUrl, err := GetParam[string](ctx, ProjectUrlParam)
		if err != nil {
			return err
		}
		environment, err := GetParam[string](ctx, EnvironmentParam)
		if err != nil {
			return err
		}
		step, err := GetParam[string](ctx, StepParam)
		if err != nil {
			return err
		}
		at, err := ParseEvidenceTime(evidence.Current)
		if err != nil {
			return err
		}
		return w.time.Set(projectUrl, environment, step, at)
	}

	return fmt.Errorf("no hay dónde guardar la evidencia de la regla %q", evidence.RuleName)
}

// projectPipelineStepKey es la clave de las tres reglas que sí llevan pipeline.
func projectPipelineStepKey(ctx RuleContext) (projectUrl, pipelineUrl, step string, err error) {
	if projectUrl, err = GetParam[string](ctx, ProjectUrlParam); err != nil {
		return "", "", "", err
	}
	if pipelineUrl, err = GetParam[string](ctx, PipelineUrlParam); err != nil {
		return "", "", "", err
	}
	if step, err = GetParam[string](ctx, StepParam); err != nil {
		return "", "", "", err
	}
	return projectUrl, pipelineUrl, step, nil
}
