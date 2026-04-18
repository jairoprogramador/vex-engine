package dto

type CommandDTO struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description,omitempty"`
	Cmd           string   `yaml:"cmd"`
	Workdir       string   `yaml:"workdir,omitempty"`
	TemplateFiles []string `yaml:"templates,omitempty"`
	Outputs       []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description,omitempty"`
		Probe       string `yaml:"probe"`
	} `yaml:"outputs,omitempty"`
}
