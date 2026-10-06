package dominio

// Ambito es compartido o el de un ambiente concreto — el mismo concepto que resolucion/publicado.Ambito y
// historial/publicado.Ambito, en el lenguaje de este dominio: los tres puertos que lo usan (Variables,
// Historial) lo entienden así.
type Ambito struct {
	compartido bool
	ambiente   string
}

func AmbitoDeAmbiente(ambiente string) (Ambito, error) {
	if ambiente == "" {
		return Ambito{}, invalido("un ámbito de ambiente no puede tener el nombre vacío")
	}
	return Ambito{ambiente: ambiente}, nil
}

func AmbitoCompartido() Ambito { return Ambito{compartido: true} }

func (a Ambito) EsCompartido() bool { return a.compartido }

func (a Ambito) Ambiente() string { return a.ambiente }

func (a Ambito) String() string {
	if a.compartido {
		return "compartido"
	}
	return a.ambiente
}

// AmbitoDelPaso es el ámbito bajo el que un paso archiva su historia: el compartido si el paso lo es, o si no,
// el del ambiente en que corre.
func AmbitoDelPaso(paso PasoDelPipeline, ambiente string) (Ambito, error) {
	if paso.Compartido() {
		return AmbitoCompartido(), nil
	}
	return AmbitoDeAmbiente(ambiente)
}
