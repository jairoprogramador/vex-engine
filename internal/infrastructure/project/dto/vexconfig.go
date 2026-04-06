package dto

type VexConfigDTO struct {
	Project  ProjectDTO  `yaml:"project"`
	Template TemplateDTO `yaml:"template"`
}
