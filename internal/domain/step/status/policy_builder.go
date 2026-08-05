package status

import (
	"errors"
	"fmt"
	"strings"
)

var PolicyTestRulesNames = []string{
	InstPipelineRuleName,
	VariablesRuleName,
	CodeProjectRuleName,
	TimeRuleName,
}

var PolicySupplyRulesNames = []string{
	InstPipelineRuleName,
	VariablesRuleName,
}

var PolicyPackageRulesNames = []string{
	InstPipelineRuleName,
	VariablesRuleName,
	CodeProjectRuleName,
}

var PolicyDeployRulesNames = []string{
	InstPipelineRuleName,
	VariablesRuleName,
	CodeProjectRuleName,
}

// DUPLICACIÓN CONOCIDA (spec 05 §5.1'): estas cuatro constantes son las mismas
// cuatro que `command.StepNameValue` declara como tipo propio. El mismo
// vocabulario tiene dos representaciones sin relación de tipos, y nadie las
// mantiene sincronizadas.
//
// No se unifican aquí a propósito: la spec 10 borra este paquete entero al
// sustituir las cuatro reglas por una comparación de cache_key, y con él este
// bloque y el switch de abajo. Unificarlos ahora sería dar por buena una casa
// que se va a demoler. Queda marcado para que no se lea como descuido.
const (
	StepTest    = "test"
	StepSupply  = "supply"
	StepPackage = "package"
	StepDeploy  = "deploy"
)

// KnownSteps son los steps cuyas comprobaciones el motor tiene cableadas. Es el
// vocabulario CERRADO de la decisión de re-ejecución, frente al vocabulario
// ABIERTO con el que `command.NewStepName` acepta cualquier `NN-<nombre>`.
// Existe para que el mensaje de error los enumere sin volver a escribirlos.
var KnownSteps = []string{StepTest, StepSupply, StepPackage, StepDeploy}

type PolicyBuilder struct {
	registry *RuleRegistry
	errs     []error
}

func NewPolicyBuilder(registry *RuleRegistry) *PolicyBuilder {
	return &PolicyBuilder{
		registry: registry,
	}
}

func (b *PolicyBuilder) Build(stepName string) (*Policy, error) {
	// CÓDIGO MUERTO: nada escribe nunca en `b.errs` — el builder no tiene fase de
	// acumulación. Se deja porque no es de esta spec, pero conviene no leerlo como
	// una vía de fallo viva: la única de este método es el step desconocido.
	if len(b.errs) > 0 {
		return nil, errors.Join(b.errs...)
	}

	ruleNames, err := b.getArrayRulesNames(stepName)
	if err != nil {
		return nil, err
	}

	rules := make([]Rule, 0, len(ruleNames))
	for _, ruleName := range ruleNames {
		rule, err := b.registry.Get(ruleName)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return NewPolicy(stepName, rules...), nil
}

// getArrayRulesNames devuelve las comprobaciones cableadas para un step, y
// ERROR si el motor no conoce ese step (spec 05 §5.2).
//
// Un step desconocido no es «un step sin reglas»: es un step que el motor no
// sabe evaluar, y hoy el autor del pipelinecode no tiene forma de expresar su
// intención. Devolver una lista vacía hacía que se saltara para siempre; devolver
// todas las reglas sería adivinar —le aplicaría el TTL de 30 días y la huella de
// código a un step del que no se sabe nada—. Por eso falla.
//
// MEDIDA DE TRANSICIÓN, y corta a propósito (spec 05 §9.5). El switch entero
// desaparece en la spec 10, que borra PolicyBuilder al sustituir las reglas por
// una comparación de cache_key: desde ahí un step desconocido no tiene entrada de
// caché y por tanto **se ejecuta**. La spec 15 no abre el vocabulario —ya está
// abierto—, le devuelve la granularidad con `checks` declarado por step.
//
// Fallar es correcto ahora precisamente porque el autor todavía no tiene forma de
// escribir «este paso no tiene nada que comprobar».
func (b *PolicyBuilder) getArrayRulesNames(stepName string) ([]string, error) {
	switch stepName {
	case StepTest:
		return PolicyTestRulesNames, nil
	case StepSupply:
		return PolicySupplyRulesNames, nil
	case StepPackage:
		return PolicyPackageRulesNames, nil
	case StepDeploy:
		return PolicyDeployRulesNames, nil
	}
	return nil, fmt.Errorf(
		"el motor no sabe cómo decidir si el paso '%s' debe re-ejecutarse: no tiene comprobaciones definidas. Los pasos conocidos son: %s",
		stepName, strings.Join(KnownSteps, ", "))
}
