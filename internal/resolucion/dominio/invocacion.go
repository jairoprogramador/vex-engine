package dominio

// VariablesDeUnaInvocacion es el agregado: las variables efectivas que se van acumulando mientras dura una
// invocación (DEC-08.3) — un intento real o una petición sin intento (DEC-04.10), el dominio no distingue
// entre las dos. Una declarada nunca gana a lo que ya hay con su nombre; una producida siempre gana, y llamar
// Producir en el orden en que se ejecutan los comandos basta para que "entre producidas gane la más reciente"
// (DEC-08.4).
type VariablesDeUnaInvocacion struct {
	porNombre map[string]VariableEfectiva
	orden     []string
}

func NuevaInvocacion() *VariablesDeUnaInvocacion {
	return &VariablesDeUnaInvocacion{porNombre: map[string]VariableEfectiva{}}
}

// Declarar añade un literal o una variable estándar. Si el nombre ya está —fuera cual fuera su origen— no
// hace nada: una declarada nunca sobreescribe.
func (v *VariablesDeUnaInvocacion) Declarar(variable VariableEfectiva) error {
	if variable.Origen() != OrigenDeclarada {
		return rechazo("%q: declarar exige una variable de origen declarada", variable.Nombre())
	}
	if _, ya := v.porNombre[variable.Nombre()]; ya {
		return nil
	}
	v.agregar(variable)
	return nil
}

// Producir añade o sustituye lo que produjo un comando, o lo que aportó la última vez un paso que no se
// re-ejecuta. Siempre gana, sobre una declarada o sobre una producida anterior.
func (v *VariablesDeUnaInvocacion) Producir(variable VariableEfectiva) error {
	if variable.Origen() != OrigenProducida {
		return rechazo("%q: producir exige una variable de origen producida", variable.Nombre())
	}
	v.agregar(variable)
	return nil
}

func (v *VariablesDeUnaInvocacion) agregar(variable VariableEfectiva) {
	if _, ya := v.porNombre[variable.Nombre()]; !ya {
		v.orden = append(v.orden, variable.Nombre())
	}
	v.porNombre[variable.Nombre()] = variable
}

// Visibles son las variables que se ven desde ambito, en el orden en que se agregaron por primera vez.
func (v *VariablesDeUnaInvocacion) Visibles(ambito Ambito) []VariableEfectiva {
	var visibles []VariableEfectiva
	for _, nombre := range v.orden {
		variable := v.porNombre[nombre]
		if variable.Ambito().Ve(ambito) {
			visibles = append(visibles, variable)
		}
	}
	return visibles
}
