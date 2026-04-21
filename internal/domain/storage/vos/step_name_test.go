package vos

import "testing"

func TestNewStepName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		want    StepName
	}{
		{"test válido", "test", false, StepTest},
		{"supply válido", "supply", false, StepSupply},
		{"package válido", "package", false, StepPackage},
		{"deploy válido", "deploy", false, StepDeploy},
		{"inválido", "build", true, ""},
		{"vacío", "", true, ""},
		{"mayúsculas", "Test", true, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewStepName(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("esperaba error para %q, no lo obtuvo", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if got != tc.want {
				t.Errorf("esperaba %q, obtuvo %q", tc.want, got)
			}
		})
	}
}
