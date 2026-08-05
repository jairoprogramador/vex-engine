package status

const (
	PipelineUrlParam = "pipeline_url"
	ProjectUrlParam  = "project_url"
	EnvironmentParam = "environment"
	StepParam        = "step"
)

type Rule interface {
	Name() string

	// Evaluate es una CONSULTA: mira el estado y responde, pero NO lo modifica
	// (spec 09 §5.1). Hasta la spec 09 este método escribía la huella nueva en
	// disco en cuanto detectaba un cambio —un nombre de consulta con efecto de
	// comando—, y de ahí salían los tres defectos: el efecto ocurría en el
	// momento equivocado, el fallo del efecto contaminaba el valor de retorno, y
	// los dos canales de respuesta se mezclaban en uno.
	//
	// Las reglas siguen recibiendo su repositorio para LEER. La spec 10 se lo
	// quita también, al sustituir las cuatro por una comparación de `cache_key`.
	//
	// La evidencia va en PLURAL aunque la spec la dibuje en singular: `Policy`
	// implementa `Rule` —es el Composite a medias que la spec 05 anotó y que la
	// 10 elimina por sustitución— y la observación de un compuesto es un
	// conjunto. Una regla hoja devuelve exactamente un elemento; con eso el
	// anidamiento sigue en pie sin construir aquí un Composite que la 10 borra.
	Evaluate(ctx RuleContext) (Decision, []Evidence, error)
}
