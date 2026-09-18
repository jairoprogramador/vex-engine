package dominio

// Eje es código, instrucciones o variables — lista cerrada: no hay un cuarto (DEC-01.2).
type Eje string

const (
	Codigo        Eje = "codigo"
	Instrucciones Eje = "instrucciones"
	Variables     Eje = "variables"
)

// TodosLosEjes, en su orden canónico — un orden de presentación, nunca de rango (invariante «nunca dice
// probablemente»).
func TodosLosEjes() []Eje { return []Eje{Codigo, Instrucciones, Variables} }

func (e Eje) String() string { return string(e) }

// Ambiente es la separación en la que ocurrió un intento.
type Ambiente struct{ valor string }

func NuevaAmbiente(valor string) (Ambiente, error) {
	if valor == "" {
		return Ambiente{}, invalido("un ambiente no puede tener el nombre vacío")
	}
	return Ambiente{valor: valor}, nil
}

func (a Ambiente) String() string { return a.valor }

// IdIntento identifica, ante Diagnóstico, un intento del Historial. Es opaco: Diagnóstico nunca lo
// interpreta, solo lo lleva.
type IdIntento struct{ valor string }

func NuevoIdIntento(valor string) (IdIntento, error) {
	if valor == "" {
		return IdIntento{}, invalido("un intento no puede tener la identidad vacía")
	}
	return IdIntento{valor: valor}, nil
}

func (i IdIntento) String() string { return i.valor }

// IdDespliegue identifica, ante Diagnóstico, un despliegue del Historial. Opaco, igual que IdIntento.
type IdDespliegue struct{ valor string }

func NuevoIdDespliegue(valor string) (IdDespliegue, error) {
	if valor == "" {
		return IdDespliegue{}, invalido("un despliegue no puede tener la identidad vacía")
	}
	return IdDespliegue{valor: valor}, nil
}

func (d IdDespliegue) String() string { return d.valor }

// IdLanzamiento identifica, ante Diagnóstico, un lanzamiento del Historial. Opaco, igual que IdIntento.
type IdLanzamiento struct{ valor string }

func NuevoIdLanzamiento(valor string) (IdLanzamiento, error) {
	if valor == "" {
		return IdLanzamiento{}, invalido("un lanzamiento no puede tener la identidad vacía")
	}
	return IdLanzamiento{valor: valor}, nil
}

func (l IdLanzamiento) String() string { return l.valor }

// NombrePaso es el nombre de un paso del pipeline, sin su prefijo de orden.
type NombrePaso struct{ valor string }

func NuevoNombrePaso(valor string) (NombrePaso, error) {
	if valor == "" {
		return NombrePaso{}, invalido("un paso no puede tener el nombre vacío")
	}
	return NombrePaso{valor: valor}, nil
}

func (p NombrePaso) String() string { return p.valor }

// NombreDeVariable es el nombre de una variable, declarada o producida. Nunca lleva su valor.
type NombreDeVariable struct{ valor string }

func NuevoNombreDeVariable(valor string) (NombreDeVariable, error) {
	if valor == "" {
		return NombreDeVariable{}, invalido("una variable no puede tener el nombre vacío")
	}
	return NombreDeVariable{valor: valor}, nil
}

func (n NombreDeVariable) String() string { return n.valor }

// HashDelCodigo es el hash del código con el que se hizo de verdad un paso.
type HashDelCodigo struct{ valor string }

func NuevoHashDelCodigo(valor string) (HashDelCodigo, error) {
	if valor == "" {
		return HashDelCodigo{}, invalido("un hash del código no puede estar vacío")
	}
	return HashDelCodigo{valor: valor}, nil
}

func (h HashDelCodigo) String() string { return h.valor }

// HashDeInstrucciones es el hash de las instrucciones con las que se hizo de verdad un paso (DEC-08.7):
// nunca el del pipeline entero, que mezcla los tres ejes (DEC-06.9).
type HashDeInstrucciones struct{ valor string }

func NuevoHashDeInstrucciones(valor string) (HashDeInstrucciones, error) {
	if valor == "" {
		return HashDeInstrucciones{}, invalido("un hash de instrucciones no puede estar vacío")
	}
	return HashDeInstrucciones{valor: valor}, nil
}

func (h HashDeInstrucciones) String() string { return h.valor }

// HashDeVariable es el hash de una variable, comparable entre ambientes de un mismo proyecto (DEC-08.6).
// Nunca lleva el valor en claro.
type HashDeVariable struct{ valor string }

func NuevoHashDeVariable(valor string) (HashDeVariable, error) {
	if valor == "" {
		return HashDeVariable{}, invalido("un hash de variable no puede estar vacío")
	}
	return HashDeVariable{valor: valor}, nil
}

func (h HashDeVariable) String() string { return h.valor }
