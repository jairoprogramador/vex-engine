package step

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// PipelineStepConfigDTO es la forma en disco de `steps/NN-nombre/config.yaml`.
//
// Dos claves, y son dos preguntas distintas sobre el mismo objeto:
//
//	scope: project                 # ← spec 13: DÓNDE vive el estado
//
//	rules:                         # ← spec 15: CUÁNDO deja de ser válido
//	  - state_changed: [pipeline]
//	  - max_age: 24h
//
// Por esto el archivo existe como tal en vez de ser un `scope.txt`: lo que el
// step declara sobre sí mismo iba a crecer, y crece en un sitio. Lo que NO se
// hizo es meter las reglas en `commands.yaml` —que es una LISTA, y una cabecera
// lo convertiría en «una lista, o un mapa con una lista dentro»— ni en un
// `policy/<step>.yaml` aparte, que sería un tercer archivo por step para
// describir una sola unidad.
//
// La versión de formato es la del pipelinecode entero: `vexpipeline.yaml` con
// `schema_version: 2` (spec 14 §5.1) cubre también este archivo, así que `rules`
// no negocia una versión propia.
type PipelineStepConfigDTO struct {
	Scope string `yaml:"scope"`

	// Rules ausente y `rules: []` significan lo mismo, y es deliberado: en los
	// dos casos no hay nada que comprobar, el step se ejecuta siempre y no
	// escribe registro (spec 15 §5.5). Distinguirlos exigiría que el motor
	// adivinara una intención a partir de una ausencia.
	Rules []PipelineStepRuleDTO `yaml:"rules"`
}

// PipelineStepRuleDTO es la forma en disco de UN elemento de `rules`, y son dos
// formas en el mismo archivo:
//
//   - state_changed                        forma corta, sin parámetros
//   - state_changed: [pipeline, project]   forma larga, con los suyos
//   - max_age: 24h
//
// El DTO se queda en la FORMA —qué regla se nombró y con qué nodo detrás— y no
// decide nada sobre el significado. Traducir a `step.Rule` es del repositorio,
// que es donde vive el vocabulario cerrado, igual que `scope` desde la spec 13.
type PipelineStepRuleDTO struct {
	// Name es la clave con la que la regla se nombró, tal cual se escribió.
	Name string

	// Shorthand distingue `- state_changed` de `- state_changed: [...]`, y hace
	// falta: la forma corta tiene un significado propio —«vigílalo todo»— que no
	// es el mismo que una lista vacía, que dice «no vigiles nada».
	Shorthand bool

	// Value es el nodo con los parámetros de la regla, sin decodificar. Cada
	// regla toma lo suyo: `state_changed` una lista de fuentes, `max_age` una
	// duración.
	Value yaml.Node
}

// UnmarshalYAML acepta las dos formas y rechaza todo lo demás nombrando lo que
// se esperaba.
//
// Un mapa con más de una clave —`- {state_changed: [pipeline], max_age: 24h}`—
// es un error y no dos reglas: la lista existe para declarar reglas, y aceptar
// las dos escrituras haría que el orden de evaluación (y con él el motivo que se
// emite) dependiera del orden interno de un mapa YAML.
func (d *PipelineStepRuleDTO) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag != "!!str" || node.Value == "" {
			return fmt.Errorf(
				"un elemento de 'rules' es el nombre de una regla o un mapa 'regla: parámetros'")
		}
		d.Name, d.Shorthand = node.Value, true
		return nil

	case yaml.MappingNode:
		if len(node.Content) != 2 {
			return fmt.Errorf(
				"cada elemento de 'rules' declara UNA regla, y éste declara %d",
				len(node.Content)/2)
		}
		d.Name, d.Value = node.Content[0].Value, *node.Content[1]
		return nil

	default:
		return fmt.Errorf(
			"un elemento de 'rules' es el nombre de una regla o un mapa 'regla: parámetros'")
	}
}
