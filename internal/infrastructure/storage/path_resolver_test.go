package storage

import (
	"path/filepath"
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

func TestDefaultPathResolver_Resolve(t *testing.T) {
	resolver := NewDefaultPathResolver("/var/lib/vexd")
	key := vos.NewStorageKey("myproject", "mytemplate", vos.StepDeploy)

	got := resolver.Resolve(key)
	expected := filepath.Join("/var/lib/vexd", "myproject", "mytemplate", "storage", "deploy.tb")

	if got != expected {
		t.Errorf("ruta incorrecta:\n  esperada: %s\n  obtenida: %s", expected, got)
	}
}

func TestDefaultPathResolver_AllSteps(t *testing.T) {
	resolver := NewDefaultPathResolver("/root")
	steps := []vos.StepName{vos.StepTest, vos.StepSupply, vos.StepPackage, vos.StepDeploy}

	for _, step := range steps {
		t.Run(string(step), func(t *testing.T) {
			key := vos.NewStorageKey("p", "t", step)
			got := resolver.Resolve(key)
			expected := filepath.Join("/root", "p", "t", "storage", step.String()+".tb")
			if got != expected {
				t.Errorf("esperada %s, obtenida %s", expected, got)
			}
		})
	}
}
