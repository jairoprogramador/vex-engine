package services

import (
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

func TestDefaultCatalog_AllStepsPresent(t *testing.T) {
	catalog := DefaultCatalog()

	steps := []vos.StepName{vos.StepTest, vos.StepSupply, vos.StepPackage, vos.StepDeploy}
	for _, step := range steps {
		t.Run(string(step), func(t *testing.T) {
			policy, err := catalog.Lookup(step)
			if err != nil {
				t.Fatalf("paso %q no encontrado en el catálogo: %v", step, err)
			}
			if policy.Rule() == nil {
				t.Errorf("la política del paso %q tiene rule nil", step)
			}
			if policy.Step() != step {
				t.Errorf("step mismatch: esperaba %q, obtuvo %q", step, policy.Step())
			}
		})
	}
}

func TestDefaultCatalog_UnknownStep(t *testing.T) {
	catalog := DefaultCatalog()
	_, err := catalog.Lookup("build")
	if err == nil {
		t.Error("se esperaba error para paso desconocido")
	}
}
