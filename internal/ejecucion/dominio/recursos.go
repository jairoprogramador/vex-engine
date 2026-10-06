package dominio

// HashDeCodigo es el hash del código del proyecto que un paso usa (lo calcula Suministro). Solo envuelve y
// compara un valor ya calculado; este dominio no lo calcula.
type HashDeCodigo struct{ valor string }

func NuevoHashDeCodigo(valor string) (HashDeCodigo, error) {
	if valor == "" {
		return HashDeCodigo{}, invalido("un hash de código no puede estar vacío")
	}
	return HashDeCodigo{valor: valor}, nil
}

func (h HashDeCodigo) String() string { return h.valor }

// HashDeInstrucciones es el hash de las instrucciones de un paso: sus comandos y el material de su directorio,
// nunca su configuración (DEC-08.7). Lo calcula la infraestructura de Ejecución.
type HashDeInstrucciones struct{ valor string }

func NuevoHashDeInstrucciones(valor string) (HashDeInstrucciones, error) {
	if valor == "" {
		return HashDeInstrucciones{}, invalido("un hash de instrucciones no puede estar vacío")
	}
	return HashDeInstrucciones{valor: valor}, nil
}

func (h HashDeInstrucciones) String() string { return h.valor }

// HashDeVariables es el hash de las variables que un paso consume: las del pipeline de su ámbito, las que
// produjeron pasos anteriores y los metadatos — nunca las generadas por el motor. Lo calcula Resolución; este
// dominio solo lo guarda con el paso y lo compara. Puede estar vacío: es lo que deja un registro escrito antes de
// que el paso lo guardara, y por eso nunca coincide con uno calculado — el paso se reejecuta una vez.
type HashDeVariables struct{ valor string }

func NuevoHashDeVariables(valor string) HashDeVariables { return HashDeVariables{valor: valor} }

func (h HashDeVariables) String() string { return h.valor }

// RecursosDeUnPaso es lo que un paso usa para decidir si se re-ejecuta: el hash del código, el hash de sus
// instrucciones y el de las variables que consume. Cada uno lo calcula su dueño (DEC-09.3): el código,
// Suministro; las instrucciones, la infraestructura de este contexto; las variables, Resolución. Los tres se
// guardan con el paso y la próxima vez se comparan por igual.
type RecursosDeUnPaso struct {
	hashDelCodigo       HashDeCodigo
	hashDeInstrucciones HashDeInstrucciones
	hashDeVariables     HashDeVariables
}

func NuevosRecursosDeUnPaso(
	hashDelCodigo HashDeCodigo, hashDeInstrucciones HashDeInstrucciones, hashDeVariables HashDeVariables,
) RecursosDeUnPaso {
	return RecursosDeUnPaso{
		hashDelCodigo: hashDelCodigo, hashDeInstrucciones: hashDeInstrucciones, hashDeVariables: hashDeVariables,
	}
}

func (r RecursosDeUnPaso) HashDelCodigo() HashDeCodigo { return r.hashDelCodigo }

func (r RecursosDeUnPaso) HashDeInstrucciones() HashDeInstrucciones { return r.hashDeInstrucciones }

func (r RecursosDeUnPaso) HashDeVariables() HashDeVariables { return r.hashDeVariables }
