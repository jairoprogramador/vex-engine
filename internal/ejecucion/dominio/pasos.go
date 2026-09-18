package dominio

// PasoDelPipeline es un paso, en su lugar del orden del pipeline, con su ámbito (compartido, o el del ambiente
// en que se ejecuta). Es lo mínimo para que el Intento en curso sepa el orden y para abrir en el Historial
// (historial/publicado.PasoDeclarado).
type PasoDelPipeline struct {
	nombre     string
	compartido bool
}

func NuevoPasoDelPipeline(nombre string, compartido bool) (PasoDelPipeline, error) {
	if nombre == "" {
		return PasoDelPipeline{}, invalido("un paso del pipeline no puede tener el nombre vacío")
	}
	return PasoDelPipeline{nombre: nombre, compartido: compartido}, nil
}

func (p PasoDelPipeline) Nombre() string { return p.nombre }

func (p PasoDelPipeline) Compartido() bool { return p.compartido }

func (p PasoDelPipeline) String() string { return p.nombre }
