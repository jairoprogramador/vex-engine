package vos

// StorageKey identifica de forma única el historial de ejecución de un paso
// para un proyecto y pipeline concretos, sin asumir nada sobre la persistencia.
type StorageKey struct {
	projectName  string
	templateName string
	step         StepName
}

func NewStorageKey(projectName, templateName string, step StepName) StorageKey {
	return StorageKey{
		projectName:  projectName,
		templateName: templateName,
		step:         step,
	}
}

func (k StorageKey) ProjectName() string  { return k.projectName }
func (k StorageKey) TemplateName() string { return k.templateName }
func (k StorageKey) Step() StepName       { return k.step }

func (k StorageKey) Equals(other StorageKey) bool {
	return k.projectName == other.projectName &&
		k.templateName == other.templateName &&
		k.step == other.step
}
