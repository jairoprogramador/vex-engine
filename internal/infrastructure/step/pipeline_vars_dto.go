package step

// PipelineVariableDTO es la forma en disco de `variables/<ambiente>/<paso>.yaml`.
//
// Era exactamente `{name, description, value}` y por eso el consumidor no podía
// declarar de dónde viene un valor: no había campo donde ponerlo (spec 14 §1).
// Los cuatro nuevos son la gramática de §5.2, y son OPCIONALES en el DTO a
// propósito — quién exige cuál lo decide `resolve`, y eso es una regla del
// dominio, no del parser.
//
//	# (a) literal — sigue existiendo, y sigue rigiéndose por la spec 12
//	- name: "instance_count"
//	  value: "3"
//
//	# (b) producido por el output de un step de este pipeline
//	- name: "acr_login_server"
//	  resolve: step-output
//	  from: "02-supply"
//	  key: "azure_container_registry_login_server"
//
//	# (c) leído del registro de estado de otro ámbito
//	- name: "DB_HOST"
//	  resolve: state
//	  scope: project
//	  key: "lb-arn"
type PipelineVariableDTO struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Value       string `yaml:"value,omitempty"`

	// Resolve es el vocabulario cerrado de §5.2. Ausente ⇒ literal.
	Resolve string `yaml:"resolve,omitempty"`
	From    string `yaml:"from,omitempty"`
	Key     string `yaml:"key,omitempty"`
	Scope   string `yaml:"scope,omitempty"`
}
