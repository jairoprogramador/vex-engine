package dominio

import (
	"slices"
	"strings"
)

// detectadorCirculos busca ciclos de referencias entre variables declaradas dentro de un ámbito: un valor que,
// para resolverse, termina necesitando su propio valor. Se ejecuta una vez por ámbito, porque lo que se ve
// desde uno no se ve desde otro.
type detectadorCirculos struct {
	comprobacion *comprobacion
	ambito       Ambito
	literales    map[string]variableDePipelineEnComprobacion
	estado       map[string]int
	camino       []string
}

const (
	sinVisitar = iota
	enCamino
	terminado
)

func (det *detectadorCirculos) detectar() {
	det.cargarLiterales()
	for _, variable := range det.comprobacion.variablesDePipeline {
		if det.esVariableNoVisitada(variable) {
			det.camino = det.camino[:0]
			det.visitar(variable.Nombre)
		}
	}
}

func (det *detectadorCirculos) cargarLiterales() {
	for _, v := range det.comprobacion.variablesDePipeline {
		if det.ambito.Ve(v.Ambito) {
			det.literales[v.Nombre] = v
		}
	}
}

func (det *detectadorCirculos) esVariableNoVisitada(v variableDePipelineEnComprobacion) bool {
	_, esLiteral := det.literales[v.Nombre]
	return esLiteral && det.estado[v.Nombre] == sinVisitar && v.Ambito == det.ambito
}

func (det *detectadorCirculos) visitar(nombre string) bool {
	det.estado[nombre] = enCamino
	det.camino = append(det.camino, nombre)

	nombres, _ := usos(det.literales[nombre].Valor)
	for _, usado := range nombres {
		if !det.esLiteralVisible(usado) {
			continue
		}

		switch det.estado[usado] {
		case enCamino:
			if det.debeReportarCirculo(usado) {
				det.reportarCirculo(nombre, usado)
			}
			return true
		case sinVisitar:
			if det.visitar(usado) {
				return true
			}
		}
	}

	det.camino = det.camino[:len(det.camino)-1]
	det.estado[nombre] = terminado
	return false
}

func (det *detectadorCirculos) esLiteralVisible(nombre string) bool {
	_, es := det.literales[nombre]
	return es
}

func (det *detectadorCirculos) debeReportarCirculo(usado string) bool {
	if det.ambito.EsCompartido() {
		return true
	}
	return tocaElAmbiente(det.caminoDesde(usado), det.literales)
}

func (det *detectadorCirculos) caminoDesde(nombre string) []string {
	inicio := slices.Index(det.camino, nombre)
	return det.camino[inicio:]
}

func (det *detectadorCirculos) reportarCirculo(nombre, usado string) {
	camino := det.caminoDesde(usado)
	det.comprobacion.falla(Fallo{Invariante: Variables, Fichero: det.literales[nombre].fichero, Ambiente: det.ambito.deUnAmbiente()},
		"las variables se usan en círculo: %s → %s", strings.Join(camino, " → "), usado)
}

// tocaElAmbiente dice si un círculo pasa por alguna variable que no es compartida.
func tocaElAmbiente(circulo []string, literales map[string]variableDePipelineEnComprobacion) bool {
	return slices.ContainsFunc(circulo, func(n string) bool { return !literales[n].Ambito.EsCompartido() })
}
