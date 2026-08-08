package pipeline

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

const (
	StepEntryFormatRuleName       = "formato_del_directorio"
	UniqueStepOrderRuleName       = "orden_único"
	StepConfigRuleName            = "configuración_del_step"
	DeclaredSourceVersionRuleName = "versión_del_formato"
	VariableGraphRuleName         = "grafo_de_variables"
)

// StepEntryFormatRule exige que todo directorio bajo `steps/` case
// `^\d{2}-.+$`. Nombra en el error cada directorio culpable: hasta la spec 04
// un `4_test` desaparecía del pipeline sin un mensaje y un `2-supply` pasaba sin
// ejecutar nada (spec 04 §1 b′, c).
type StepEntryFormatRule struct{}

var _ StepStructureRule = (*StepEntryFormatRule)(nil)

func NewStepEntryFormatRule() StepEntryFormatRule {
	return StepEntryFormatRule{}
}

func (r StepEntryFormatRule) Name() string { return StepEntryFormatRuleName }

func (r StepEntryFormatRule) IsSatisfiedBy(_ *context.Context, _ Pipelinecode, entries []StepEntry) error {
	errs := make([]error, 0, len(entries))
	for _, entry := range entries {
		if _, err := command.NewStepName(entry.String()); err != nil {
			errs = append(errs, fmt.Errorf(
				"el directorio 'steps/%s' no es un step válido: se espera '%s'",
				entry, command.StepDirNameFormat))
		}
	}
	return errors.Join(errs...)
}

// UniqueStepOrderRule exige que dos steps no declaren el mismo orden: con
// `02-a` y `02-b` el orden de ejecución sería arbitrario y las dos claves de
// estado colisionarían.
//
// Los directorios que no cumplen el formato se ignoran aquí: de esos ya habla
// StepEntryFormatRule, y repetirlos sumaría ruido al mismo diagnóstico.
type UniqueStepOrderRule struct{}

var _ StepStructureRule = (*UniqueStepOrderRule)(nil)

func NewUniqueStepOrderRule() UniqueStepOrderRule {
	return UniqueStepOrderRule{}
}

func (r UniqueStepOrderRule) Name() string { return UniqueStepOrderRuleName }

func (r UniqueStepOrderRule) IsSatisfiedBy(_ *context.Context, _ Pipelinecode, entries []StepEntry) error {
	byOrder := make(map[int][]string, len(entries))
	for _, entry := range entries {
		stepName, err := command.NewStepName(entry.String())
		if err != nil {
			continue
		}
		byOrder[stepName.Order()] = append(byOrder[stepName.Order()], entry.String())
	}

	duplicated := make([]int, 0)
	for order, names := range byOrder {
		if len(names) > 1 {
			duplicated = append(duplicated, order)
		}
	}
	slices.Sort(duplicated)

	errs := make([]error, 0, len(duplicated))
	for _, order := range duplicated {
		names := byOrder[order]
		slices.Sort(names)
		errs = append(errs, fmt.Errorf(
			"el orden %02d lo declaran varios directorios: %s",
			order, strings.Join(quoted(names), ", ")))
	}
	return errors.Join(errs...)
}

// StepConfigRule exige que todo `config.yaml` PRESENTE sea legible y esté
// escrito con la gramática que el motor conoce: un `scope` del vocabulario
// cerrado (spec 13 §5.1) y unas `rules` del suyo (spec 15 §5.2).
//
// **La regla no crece con la gramática, y eso es la mitad del argumento.** No
// enumera campos ni conoce reglas de re-ejecución: llama al repositorio, que
// valida AL TRADUCIR. Así, una clave `rules` con una regla desconocida o un
// `state_changed: [project]` abortan antes del primer step sin que aquí haya que
// escribir nada — basta con que la gramática falle en el adaptador.
//
// Se llamaba `StepScopeRule` hasta la spec 15. El cambio de nombre no es
// cosmético: el nombre de la regla encabeza el mensaje del validador, y un error
// de `rules` reportado bajo `[ámbito_declarado]` estaría mintiendo sobre qué se
// comprobó.
//
// Lo que NO exige es que el archivo exista: un step sin `config.yaml` es
// legítimo —se ejecuta siempre y no persiste registro (13 §5.3)— y por eso el
// repositorio devuelve ausencia sin error. La regla sólo se pronuncia sobre lo
// que alguien escribió.
//
// Lo que TAMPOCO hace es advertir. La configuración válida y peligrosa de la
// spec 15 §5.4 —`max_age` sin `state_changed`— no cabe aquí, y no por falta de
// sitio: el veredicto de este puerto es un `error` y sólo sabe abortar, y eso no
// debe abortar. Se emite desde el handler que carga la configuración.
//
// Corre aquí y no en la cadena de step porque un `config.yaml` mal escrito
// descubierto a mitad del despliegue llega tarde: los steps anteriores ya crearon
// recursos reales. Es la misma razón por la que existe el validador (spec 04 §4).
type StepConfigRule struct {
	configs domStep.StepConfigRepository
}

var _ StepStructureRule = (*StepConfigRule)(nil)

func NewStepConfigRule(configs domStep.StepConfigRepository) StepConfigRule {
	return StepConfigRule{configs: configs}
}

func (r StepConfigRule) Name() string { return StepConfigRuleName }

func (r StepConfigRule) IsSatisfiedBy(
	ctx *context.Context, code Pipelinecode, entries []StepEntry) error {

	errs := make([]error, 0, len(entries))
	for _, entry := range entries {
		// El error del repositorio ya nombra el archivo —y con él el directorio—,
		// así que no se le añade contexto: duplicarlo haría el mensaje más largo
		// sin decir nada nuevo.
		if _, err := r.configs.Get(ctx, code.LocalPath, entry.String()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// DeclaredSourceVersionRule exige que un pipelinecode que usa `resolve` declare
// `schema_version: 2` en su `vexpipeline.yaml` (spec 14 §5.1).
//
// La ausencia del manifiesto ⇒ versión 1 ⇒ `resolve` PROHIBIDO, y el mensaje
// nombra la versión que haría falta. Eso es lo que permite añadir gramática sin
// romper a nadie en silencio: un pipelinecode de siempre se ejecuta igual que
// ayer, y uno que usa lo nuevo sin declararlo se entera de por qué.
//
// Es también la regla que reporta un `variables/<ambiente>/<paso>.yaml` que no se
// puede leer o cuyos campos obligatorios faltan —`from` sin `key`, un `resolve`
// inventado—: el repositorio los rechaza al construir la declaración y el error
// sale por aquí. La regla del grafo, en cambio, se calla ante lo ilegible, para
// que un solo typo no produzca dos mensajes que dicen lo mismo.
type DeclaredSourceVersionRule struct {
	manifests    ManifestRepository
	declarations domStep.VarsPipelineRepository
}

var _ StepStructureRule = (*DeclaredSourceVersionRule)(nil)

func NewDeclaredSourceVersionRule(
	manifests ManifestRepository,
	declarations domStep.VarsPipelineRepository) DeclaredSourceVersionRule {

	return DeclaredSourceVersionRule{manifests: manifests, declarations: declarations}
}

func (r DeclaredSourceVersionRule) Name() string { return DeclaredSourceVersionRuleName }

func (r DeclaredSourceVersionRule) IsSatisfiedBy(
	ctx *context.Context, code Pipelinecode, entries []StepEntry) error {

	manifest, err := r.manifests.Get(ctx, code.LocalPath)
	if err != nil {
		return err
	}

	errs := make([]error, 0, len(entries))
	for _, entry := range entries {
		stepName, err := command.NewStepName(entry.String())
		if err != nil {
			// Del formato del directorio ya habla StepEntryFormatRule.
			continue
		}

		declarations, err := r.declarations.Get(ctx, code.LocalPath, code.Environment, stepName.Name())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if manifest.AllowsDeclaredSources() {
			continue
		}

		for _, declaration := range declarations {
			if declaration.IsLiteral() {
				continue
			}
			errs = append(errs, fmt.Errorf(
				"la variable '%s' de 'variables/%s/%s.yaml' declara 'resolve: %s', "+
					"que exige 'schema_version: %d' en '%s' (este pipelinecode está en la versión %d)",
				declaration.Name(), code.Environment, stepName.Name(), declaration.Source(),
				SchemaVersion2, manifestFileNameForMessage, manifest.SchemaVersion()))
		}
	}
	return errors.Join(errs...)
}

// manifestFileNameForMessage nombra el archivo en el mensaje. El dominio no sabe
// de archivos —el nombre real lo pone el adaptador— pero un error que dice «hay
// que declararlo» sin decir DÓNDE obliga a quien lo lee a buscar en la
// documentación, y ese es el tipo de error que esta spec existe para retirar.
const manifestFileNameForMessage = "vexpipeline.yaml"

// VariableGraphRule son las DOS validaciones de grafo de la spec 14 §5.4, y
// existen porque `from` hace explícito el grafo de dependencias entre steps:
//
//   - un `step-output` que apunta a un step POSTERIOR al consumidor —o a él
//     mismo— es un error de pipelinecode. Hoy falla en runtime, con «variable no
//     existe», después de que `test` y `supply` ya tuvieron efectos reales.
//   - un `step-output` que apunta a un `key` que NINGÚN `outputs` de ese step
//     declara es un error. Hoy falla en runtime, o peor: lo resuelve otro step
//     que casualmente declaró el mismo nombre, que es exactamente la ambigüedad
//     del espacio plano.
//
// Las dos mueven un fallo de EJECUCIÓN a un fallo de CARGA, que es la razón de
// ser del validador entero (spec 04 §4).
//
// «Posterior o él mismo»: la spec habla de posterior, y el propio step se añade
// por la misma razón. La declaración se satisface al cargar el step, antes de su
// primer comando, así que un step que se declarara a sí mismo como fuente no
// tendría el valor todavía y fallaría siempre — y fallar al cargar el
// pipelinecode es mejor que fallar a mitad del despliegue.
type VariableGraphRule struct {
	declarations domStep.VarsPipelineRepository
	commands     domStep.PipelineCommandRepository
}

var _ StepStructureRule = (*VariableGraphRule)(nil)

func NewVariableGraphRule(
	declarations domStep.VarsPipelineRepository,
	commands domStep.PipelineCommandRepository) VariableGraphRule {

	return VariableGraphRule{declarations: declarations, commands: commands}
}

func (r VariableGraphRule) Name() string { return VariableGraphRuleName }

func (r VariableGraphRule) IsSatisfiedBy(
	ctx *context.Context, code Pipelinecode, entries []StepEntry) error {

	steps := make(map[string]command.StepName, len(entries))
	for _, entry := range entries {
		stepName, err := command.NewStepName(entry.String())
		if err != nil {
			continue
		}
		steps[stepName.FullName()] = stepName
	}

	errs := make([]error, 0, len(entries))
	for _, stepName := range orderedSteps(steps) {
		declarations, err := r.declarations.Get(ctx, code.LocalPath, code.Environment, stepName.Name())
		if err != nil {
			// Lo ilegible lo reporta DeclaredSourceVersionRule: dos mensajes para
			// el mismo typo no ayudan a nadie.
			continue
		}

		for _, declaration := range declarations {
			if declaration.Source() != domStep.SourceStepOutput {
				continue
			}
			if err := r.check(ctx, code, steps, stepName, declaration); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (r VariableGraphRule) check(
	ctx *context.Context,
	code Pipelinecode,
	steps map[string]command.StepName,
	consumer command.StepName,
	declaration domStep.VariableDeclaration) error {

	producer, exists := steps[declaration.From()]
	if !exists {
		return fmt.Errorf(
			"la variable '%s' de '%s' declara 'from: %s' y no hay ningún 'steps/%s'",
			declaration.Name(), consumer.FullName(), declaration.From(), declaration.From())
	}

	if producer.Order() >= consumer.Order() {
		return fmt.Errorf(
			"la variable '%s' de '%s' declara 'from: %s', que no se ejecuta antes: "+
				"un step sólo puede consumir el output de uno anterior",
			declaration.Name(), consumer.FullName(), declaration.From())
	}

	commands, err := r.commands.Get(ctx, code.LocalPath, producer.FullName())
	if err != nil {
		return fmt.Errorf(
			"la variable '%s' de '%s' declara 'from: %s' y sus comandos no se pueden leer: %w",
			declaration.Name(), consumer.FullName(), declaration.From(), err)
	}

	for _, cmd := range commands {
		for _, output := range cmd.Outputs() {
			if output.Name() == declaration.Key() {
				return nil
			}
		}
	}
	return fmt.Errorf(
		"la variable '%s' de '%s' declara 'key: %s' y '%s' no lo declara en ningún 'outputs'",
		declaration.Name(), consumer.FullName(), declaration.Key(), declaration.From())
}

// orderedSteps recorre los steps en su orden declarado, para que dos
// pipelinecode con el mismo problema produzcan el mismo mensaje: un mapa de Go se
// recorre en orden aleatorio y el error saldría distinto en cada corrida.
func orderedSteps(steps map[string]command.StepName) []command.StepName {
	ordered := make([]command.StepName, 0, len(steps))
	for _, stepName := range steps {
		ordered = append(ordered, stepName)
	}
	slices.SortFunc(ordered, func(a, b command.StepName) int {
		return strings.Compare(a.FullName(), b.FullName())
	})
	return ordered
}

func quoted(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, fmt.Sprintf("'steps/%s'", name))
	}
	return out
}
