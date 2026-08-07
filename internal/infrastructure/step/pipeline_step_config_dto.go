package step

// PipelineStepConfigDTO es la forma en disco de `steps/NN-nombre/config.yaml`.
//
// Un solo campo hoy. La spec 15 añade aquí las reglas de re-ejecución
// (`max_age`, qué dimensiones vigila el step), y por eso el archivo existe como
// tal en vez de ser un `scope.txt`: lo que el step declara sobre sí mismo va a
// crecer, y crece en un sitio.
type PipelineStepConfigDTO struct {
	Scope string `yaml:"scope"`
}
