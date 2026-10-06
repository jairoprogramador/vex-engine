package dominio

// EjesDeUnPaso es lo que identifica cada eje, en un paso, con los recursos con que se hizo de verdad: el
// hash del código, el de sus instrucciones y el de cada variable declarada que ve. Solo hashes —nunca un
// valor (invariante «nunca muestra un valor»)— y solo declaradas —nunca una producida (invariante «una
// variable producida no es eje», DEC-06.10)—.
type EjesDeUnPaso struct {
	paso                  NombrePaso
	hashDelCodigo         HashDelCodigo
	hashDeInstrucciones   HashDeInstrucciones
	declaradas            map[NombreDeVariable]HashDeVariable
	comparadoPorEvidencia bool
}

func NuevosEjesDeUnPaso(
	paso NombrePaso, hashDelCodigo HashDelCodigo, hashDeInstrucciones HashDeInstrucciones,
	declaradas map[NombreDeVariable]HashDeVariable, comparadoPorEvidencia bool,
) EjesDeUnPaso {
	return EjesDeUnPaso{
		paso: paso, hashDelCodigo: hashDelCodigo, hashDeInstrucciones: hashDeInstrucciones,
		declaradas: declaradas, comparadoPorEvidencia: comparadoPorEvidencia,
	}
}

func (e EjesDeUnPaso) Paso() NombrePaso                         { return e.paso }
func (e EjesDeUnPaso) HashDelCodigo() HashDelCodigo             { return e.hashDelCodigo }
func (e EjesDeUnPaso) HashDeInstrucciones() HashDeInstrucciones { return e.hashDeInstrucciones }

// ComparadoPorEvidencia: el paso no se re-ejecutó y estos recursos son los del registro al que apunta su
// evidencia, que puede venir de otro ambiente (DEC-06.5, ES-4).
func (e EjesDeUnPaso) ComparadoPorEvidencia() bool { return e.comparadoPorEvidencia }

// VariableDeclarada da el hash de una variable declarada visible desde este paso, y si la tiene.
func (e EjesDeUnPaso) VariableDeclarada(nombre NombreDeVariable) (HashDeVariable, bool) {
	h, ok := e.declaradas[nombre]
	return h, ok
}

// NombresDeVariablesDeclaradas, para recorrerlas al comparar o al armar el sustento.
func (e EjesDeUnPaso) NombresDeVariablesDeclaradas() []NombreDeVariable {
	nombres := make([]NombreDeVariable, 0, len(e.declaradas))
	for n := range e.declaradas {
		nombres = append(nombres, n)
	}
	return nombres
}

// ProducidasDeUnPaso es, para un paso, el hash de cada variable producida que vio. Nunca es un eje
// (DEC-06.10), pero su cambio entra en el sustento.
type ProducidasDeUnPaso struct {
	paso       NombrePaso
	producidas map[NombreDeVariable]HashDeVariable
}

func NuevasProducidasDeUnPaso(paso NombrePaso, producidas map[NombreDeVariable]HashDeVariable) ProducidasDeUnPaso {
	return ProducidasDeUnPaso{paso: paso, producidas: producidas}
}

func (p ProducidasDeUnPaso) Paso() NombrePaso { return p.paso }

func (p ProducidasDeUnPaso) Variable(nombre NombreDeVariable) (HashDeVariable, bool) {
	h, ok := p.producidas[nombre]
	return h, ok
}

func (p ProducidasDeUnPaso) Nombres() []NombreDeVariable {
	nombres := make([]NombreDeVariable, 0, len(p.producidas))
	for n := range p.producidas {
		nombres = append(nombres, n)
	}
	return nombres
}
