package dominio

// Cambiaron compara los hashes visibles ahora contra los de la última vez: un nombre añadido, uno quitado o
// un hash distinto cuentan igual como cambio.
func Cambiaron(ahora, ultimaVez map[string]HashDeVariable) bool {
	if len(ahora) != len(ultimaVez) {
		return true
	}
	for nombre, hash := range ahora {
		if ultimaVez[nombre] != hash {
			return true
		}
	}
	return false
}
