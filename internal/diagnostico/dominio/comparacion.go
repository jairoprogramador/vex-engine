package dominio

import "sort"

// EstadoDePaso es el resultado de comparar, en un paso, los tres ejes del intento que falla contra los de
// una referencia (DEC-06.13).
type EstadoDePaso struct {
	Paso                         NombrePaso
	CambioCodigo                 bool
	CambioInstrucciones          bool
	CambioVariables              bool
	ComparadoPorEvidencia        bool
	VariablesDeclaradasCambiadas []NombreDeVariable
}

// CambioDeVariable nombra, en un paso, una variable que cambió — nunca su valor.
type CambioDeVariable struct {
	Paso   NombrePaso
	Nombre NombreDeVariable
}

// Comparacion es el intento que falla contra una referencia: el estado de cada eje en cada paso que
// alcanzó el intento. CantidadDeIntentos solo está presente para la referencia del mismo ambiente (ES-8).
type Comparacion struct {
	referencia                   Referencia
	pasos                        []EstadoDePaso
	variablesProducidasCambiadas []CambioDeVariable
	intentoDeLaReferencia        IdIntento
	cantidadDeIntentos           int
	hayCantidadDeIntentos        bool
}

func NuevaComparacion(referencia Referencia, pasos []EstadoDePaso, producidasCambiadas []CambioDeVariable) Comparacion {
	return Comparacion{referencia: referencia, pasos: pasos, variablesProducidasCambiadas: producidasCambiadas}
}

// ConCantidadDeIntentos devuelve la comparación con la cantidad de intentos ya puesta (ES-8).
func (c Comparacion) ConCantidadDeIntentos(cantidad int) Comparacion {
	c.cantidadDeIntentos = cantidad
	c.hayCantidadDeIntentos = true
	return c
}

// ConIntentoDeLaReferencia devuelve la comparación con el intento del despliegue de referencia ya puesto.
func (c Comparacion) ConIntentoDeLaReferencia(intento IdIntento) Comparacion {
	c.intentoDeLaReferencia = intento
	return c
}

func (c Comparacion) IntentoDeLaReferencia() IdIntento { return c.intentoDeLaReferencia }

func (c Comparacion) Referencia() Referencia { return c.referencia }
func (c Comparacion) Pasos() []EstadoDePaso  { return c.pasos }
func (c Comparacion) VariablesProducidasCambiadas() []CambioDeVariable {
	return c.variablesProducidasCambiadas
}
func (c Comparacion) CantidadDeIntentos() (int, bool) {
	return c.cantidadDeIntentos, c.hayCantidadDeIntentos
}

// Descarta: un eje queda descartado por esta comparación si no cambió en ninguno de sus pasos (DEC-06.11).
func (c Comparacion) Descarta(eje Eje) bool {
	for _, p := range c.pasos {
		if cambio(p, eje) {
			return false
		}
	}
	return true
}

func cambio(p EstadoDePaso, eje Eje) bool {
	switch eje {
	case Codigo:
		return p.CambioCodigo
	case Instrucciones:
		return p.CambioInstrucciones
	case Variables:
		return p.CambioVariables
	default:
		return false
	}
}

// CompararPasos compara, paso a paso, los ejes con que de verdad se hizo el intento que falla contra los
// de una referencia. Un paso presente en un solo lado cuenta como cambio de instrucciones.
func CompararPasos(delQueFalla, deLaReferencia []EjesDeUnPaso) []EstadoDePaso {
	deLaReferenciaPorPaso := make(map[NombrePaso]EjesDeUnPaso, len(deLaReferencia))
	for _, e := range deLaReferencia {
		deLaReferenciaPorPaso[e.Paso()] = e
	}
	estados := make([]EstadoDePaso, 0, len(delQueFalla))
	for _, e := range delQueFalla {
		ref, hay := deLaReferenciaPorPaso[e.Paso()]
		if !hay {
			estados = append(estados, EstadoDePaso{Paso: e.Paso(), CambioInstrucciones: true})
			continue
		}
		declaradasCambiadas := variablesDeclaradasCambiadas(e, ref)
		estados = append(estados, EstadoDePaso{
			Paso:                         e.Paso(),
			CambioCodigo:                 e.HashDelCodigo() != ref.HashDelCodigo(),
			CambioInstrucciones:          e.HashDeInstrucciones() != ref.HashDeInstrucciones(),
			CambioVariables:              len(declaradasCambiadas) > 0,
			ComparadoPorEvidencia:        e.ComparadoPorEvidencia() || ref.ComparadoPorEvidencia(),
			VariablesDeclaradasCambiadas: declaradasCambiadas,
		})
	}
	return estados
}

func variablesDeclaradasCambiadas(e, ref EjesDeUnPaso) []NombreDeVariable {
	nombres := map[NombreDeVariable]struct{}{}
	for _, n := range e.NombresDeVariablesDeclaradas() {
		nombres[n] = struct{}{}
	}
	for _, n := range ref.NombresDeVariablesDeclaradas() {
		nombres[n] = struct{}{}
	}
	var cambiadas []NombreDeVariable
	for n := range nombres {
		h1, ok1 := e.VariableDeclarada(n)
		h2, ok2 := ref.VariableDeclarada(n)
		if ok1 != ok2 || h1 != h2 {
			cambiadas = append(cambiadas, n)
		}
	}
	ordenarPorNombre(cambiadas)
	return cambiadas
}

// CompararProducidas compara, paso a paso, las variables producidas del intento que falla contra las de
// una referencia. Nunca participa en la eliminación (DEC-06.10): solo alimenta el sustento.
func CompararProducidas(delQueFalla, deLaReferencia []ProducidasDeUnPaso) []CambioDeVariable {
	refPorPaso := make(map[NombrePaso]ProducidasDeUnPaso, len(deLaReferencia))
	for _, p := range deLaReferencia {
		refPorPaso[p.Paso()] = p
	}
	var cambios []CambioDeVariable
	for _, p := range delQueFalla {
		ref, hayRef := refPorPaso[p.Paso()]
		nombres := map[NombreDeVariable]struct{}{}
		for _, n := range p.Nombres() {
			nombres[n] = struct{}{}
		}
		if hayRef {
			for _, n := range ref.Nombres() {
				nombres[n] = struct{}{}
			}
		}
		for n := range nombres {
			h1, ok1 := p.Variable(n)
			var h2 HashDeVariable
			var ok2 bool
			if hayRef {
				h2, ok2 = ref.Variable(n)
			}
			if ok1 != ok2 || h1 != h2 {
				cambios = append(cambios, CambioDeVariable{Paso: p.Paso(), Nombre: n})
			}
		}
	}
	sort.Slice(cambios, func(i, j int) bool {
		if cambios[i].Paso.String() != cambios[j].Paso.String() {
			return cambios[i].Paso.String() < cambios[j].Paso.String()
		}
		return cambios[i].Nombre.String() < cambios[j].Nombre.String()
	})
	return cambios
}

func ordenarPorNombre(nombres []NombreDeVariable) {
	sort.Slice(nombres, func(i, j int) bool { return nombres[i].String() < nombres[j].String() })
}
