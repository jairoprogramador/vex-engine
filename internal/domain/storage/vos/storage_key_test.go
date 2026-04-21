package vos

import "testing"

func TestStorageKey_Equals(t *testing.T) {
	k1 := NewStorageKey("proj", "tmpl", StepTest)
	k2 := NewStorageKey("proj", "tmpl", StepTest)
	k3 := NewStorageKey("proj", "tmpl", StepDeploy)
	k4 := NewStorageKey("other", "tmpl", StepTest)

	if !k1.Equals(k2) {
		t.Error("claves idénticas deben ser iguales")
	}
	if k1.Equals(k3) {
		t.Error("claves con distinto step no deben ser iguales")
	}
	if k1.Equals(k4) {
		t.Error("claves con distinto projectName no deben ser iguales")
	}
}

func TestStorageKey_Accessors(t *testing.T) {
	k := NewStorageKey("myproject", "mytemplate", StepSupply)
	if k.ProjectName() != "myproject" {
		t.Errorf("ProjectName: esperaba 'myproject', obtuvo %q", k.ProjectName())
	}
	if k.TemplateName() != "mytemplate" {
		t.Errorf("TemplateName: esperaba 'mytemplate', obtuvo %q", k.TemplateName())
	}
	if k.Step() != StepSupply {
		t.Errorf("Step: esperaba %q, obtuvo %q", StepSupply, k.Step())
	}
}
