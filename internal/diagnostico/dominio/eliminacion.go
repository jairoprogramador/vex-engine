package dominio

// Eliminacion es el segundo servicio de dominio (DEC-06.14): dadas una o dos comparaciones, devuelve la
// atribución. Un eje queda descartado si cualquiera de las comparaciones lo descarta (DEC-06.11). Pura:
// nunca hace I/O, recibe las comparaciones ya construidas.
func Eliminacion(comparaciones []Comparacion) Atribucion {
	descartados := map[Eje]bool{}
	for _, c := range comparaciones {
		for _, eje := range TodosLosEjes() {
			if c.Descarta(eje) {
				descartados[eje] = true
			}
		}
	}
	var candidatos []Eje
	for _, eje := range TodosLosEjes() {
		if !descartados[eje] {
			candidatos = append(candidatos, eje)
		}
	}
	return NuevaAtribucion(candidatos)
}
