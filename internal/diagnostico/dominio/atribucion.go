package dominio

// Atribucion es el conjunto de ejes candidatos — sin orden (invariante «nunca dice probablemente»): no
// hay ningún campo con el que marcar un candidato como más probable que otro.
type Atribucion struct {
	ejes map[Eje]struct{}
}

func NuevaAtribucion(ejes []Eje) Atribucion {
	conjunto := make(map[Eje]struct{}, len(ejes))
	for _, e := range ejes {
		conjunto[e] = struct{}{}
	}
	return Atribucion{ejes: conjunto}
}

func (a Atribucion) Tiene(eje Eje) bool {
	_, ok := a.ejes[eje]
	return ok
}

// Vacia: la eliminación descartó los tres ejes (ES-5) — un hecho, no un error.
func (a Atribucion) Vacia() bool { return len(a.ejes) == 0 }

// Ejes da los candidatos en el orden canónico de TodosLosEjes — de presentación, nunca de rango.
func (a Atribucion) Ejes() []Eje {
	var ejes []Eje
	for _, e := range TodosLosEjes() {
		if a.Tiene(e) {
			ejes = append(ejes, e)
		}
	}
	return ejes
}
