package services

import (
	"fmt"

	storageDomain "github.com/jairoprogramador/vex-engine/internal/domain/storage"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/services/rules"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

// StepPolicy agrupa la regla compuesta para un paso concreto.
type StepPolicy struct {
	step vos.StepName
	rule vos.Rule
}

func newPolicy(step vos.StepName, rule vos.Rule) StepPolicy {
	return StepPolicy{step: step, rule: rule}
}

// Rule retorna la regla compuesta que determina si una entrada histórica es válida.
func (p StepPolicy) Rule() vos.Rule { return p.rule }

// Step retorna el nombre del paso al que aplica esta política.
func (p StepPolicy) Step() vos.StepName { return p.step }

// StepPolicyCatalog es un mapa inmutable de StepName → StepPolicy.
type StepPolicyCatalog struct {
	policies map[vos.StepName]StepPolicy
}

func newCatalog(policies map[vos.StepName]StepPolicy) StepPolicyCatalog {
	return StepPolicyCatalog{policies: policies}
}

// Lookup retorna la StepPolicy para el paso dado.
// Retorna ErrUnknownStep si el paso no está en el catálogo.
func (c StepPolicyCatalog) Lookup(step vos.StepName) (StepPolicy, error) {
	policy, ok := c.policies[step]
	if !ok {
		return StepPolicy{}, fmt.Errorf("%w: %s", storageDomain.ErrUnknownStep, step)
	}
	return policy, nil
}

// DefaultCatalog retorna el catálogo con las políticas de los 4 pasos del sistema.
// Para agregar un paso: 1) crear la regla en services/rules/, 2) añadirla aquí.
func DefaultCatalog() StepPolicyCatalog {
	defaultTTL := vos.NewTTL(0) // 30 días por defecto

	return newCatalog(map[vos.StepName]StepPolicy{
		vos.StepTest: newPolicy(vos.StepTest, rules.AllRules(
			rules.NewFingerprintRule(vos.KindInstruction),
			rules.NewFingerprintRule(vos.KindVars),
			rules.NewFingerprintRule(vos.KindCode),
			rules.NewTTLRule(defaultTTL),
		)),
		vos.StepSupply: newPolicy(vos.StepSupply, rules.AllRules(
			rules.NewFingerprintRule(vos.KindInstruction),
			rules.NewFingerprintRule(vos.KindVars),
			rules.NewEnvironmentRule(),
		)),
		vos.StepPackage: newPolicy(vos.StepPackage, rules.AllRules(
			rules.NewFingerprintRule(vos.KindInstruction),
			rules.NewFingerprintRule(vos.KindVars),
			rules.NewFingerprintRule(vos.KindCode),
		)),
		vos.StepDeploy: newPolicy(vos.StepDeploy, rules.AllRules(
			rules.NewFingerprintRule(vos.KindInstruction),
			rules.NewFingerprintRule(vos.KindVars),
			rules.NewFingerprintRule(vos.KindCode),
			rules.NewEnvironmentRule(),
		)),
	})
}
