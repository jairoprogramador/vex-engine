package dominio

import (
	"sort"
	"time"
)

// CambioDeEje es un eje que cambió, y en qué pasos.
type CambioDeEje struct {
	Eje   Eje
	Pasos []NombrePaso
}

// Sustento es lo que se sabe de cada cambio (DEC-07.7): nunca autores ni commits, solo qué ejes cambiaron
// y en qué pasos, qué variables declaradas cambiaron (nunca su valor), qué variables producidas
// cambiaron, entre qué dos momentos, qué pasos se compararon por evidencia y las comparaciones hechas. Se
// arma únicamente a partir de comparaciones ya construidas: es una agregación pura.
type Sustento struct {
	InstanteDelIntentoQueFalla   time.Time
	EjesCambiados                []CambioDeEje
	VariablesDeclaradasCambiadas []CambioDeVariable
	VariablesProducidasCambiadas []CambioDeVariable
	PasosComparadosPorEvidencia  []NombrePaso
	Comparaciones                []Comparacion
}

func NuevoSustento(instanteDelQueFalla time.Time, comparaciones []Comparacion) Sustento {
	cambiosPorEje := map[Eje]map[string]NombrePaso{}
	var declaradas []CambioDeVariable
	declaradasVistas := map[CambioDeVariable]bool{}
	var producidas []CambioDeVariable
	producidasVistas := map[CambioDeVariable]bool{}
	pasosPorEvidencia := map[string]NombrePaso{}

	for _, c := range comparaciones {
		for _, p := range c.Pasos() {
			registrarCambio(cambiosPorEje, Codigo, p.CambioCodigo, p.Paso)
			registrarCambio(cambiosPorEje, Instrucciones, p.CambioInstrucciones, p.Paso)
			registrarCambio(cambiosPorEje, Variables, p.CambioVariables, p.Paso)
			if p.ComparadoPorEvidencia {
				pasosPorEvidencia[p.Paso.String()] = p.Paso
			}
			for _, nombre := range p.VariablesDeclaradasCambiadas {
				cv := CambioDeVariable{Paso: p.Paso, Nombre: nombre}
				if !declaradasVistas[cv] {
					declaradasVistas[cv] = true
					declaradas = append(declaradas, cv)
				}
			}
		}
		for _, cv := range c.VariablesProducidasCambiadas() {
			if !producidasVistas[cv] {
				producidasVistas[cv] = true
				producidas = append(producidas, cv)
			}
		}
	}

	var ejesCambiados []CambioDeEje
	for _, eje := range TodosLosEjes() {
		pasos, ok := cambiosPorEje[eje]
		if !ok {
			continue
		}
		ejesCambiados = append(ejesCambiados, CambioDeEje{Eje: eje, Pasos: valoresOrdenados(pasos)})
	}

	ordenarCambiosDeVariable(declaradas)
	ordenarCambiosDeVariable(producidas)

	return Sustento{
		InstanteDelIntentoQueFalla:   instanteDelQueFalla,
		EjesCambiados:                ejesCambiados,
		VariablesDeclaradasCambiadas: declaradas,
		VariablesProducidasCambiadas: producidas,
		PasosComparadosPorEvidencia:  valoresOrdenados(pasosPorEvidencia),
		Comparaciones:                comparaciones,
	}
}

func registrarCambio(cambios map[Eje]map[string]NombrePaso, eje Eje, cambio bool, paso NombrePaso) {
	if !cambio {
		return
	}
	if cambios[eje] == nil {
		cambios[eje] = map[string]NombrePaso{}
	}
	cambios[eje][paso.String()] = paso
}

func valoresOrdenados(pasos map[string]NombrePaso) []NombrePaso {
	claves := make([]string, 0, len(pasos))
	for k := range pasos {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	ordenados := make([]NombrePaso, 0, len(claves))
	for _, k := range claves {
		ordenados = append(ordenados, pasos[k])
	}
	return ordenados
}

func ordenarCambiosDeVariable(cambios []CambioDeVariable) {
	sort.Slice(cambios, func(i, j int) bool {
		if cambios[i].Paso.String() != cambios[j].Paso.String() {
			return cambios[i].Paso.String() < cambios[j].Paso.String()
		}
		return cambios[i].Nombre.String() < cambios[j].Nombre.String()
	})
}
