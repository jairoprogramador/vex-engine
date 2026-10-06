package dominio

import (
	"fmt"
	"regexp"
)

var patronDeVariable = regexp.MustCompile(`\$\{var\.([a-zA-Z0-9_]+)\}`)

// NombresUsados son los nombres de las variables que los textos referencian con ${var.<nombre>}.
func NombresUsados(textos []string) map[string]bool {
	usados := map[string]bool{}
	for _, texto := range textos {
		for _, coincidencia := range patronDeVariable.FindAllStringSubmatch(texto, -1) {
			usados[coincidencia[1]] = true
		}
	}
	return usados
}

// VariableNoEncontradaError: el texto usa un nombre que no está entre las variables disponibles.
type VariableNoEncontradaError struct{ Nombre string }

func (e *VariableNoEncontradaError) Error() string {
	return fmt.Sprintf("%v: la variable %q no está disponible para interpolar", ErrRechazado, e.Nombre)
}

func (e *VariableNoEncontradaError) Is(objetivo error) bool { return objetivo == ErrRechazado }

// Interpolar sustituye cada ${var.<nombre>} de texto por el valor de la variable de ese nombre entre
// disponibles. Falla con *VariableNoEncontradaError si algún nombre usado no está.
func Interpolar(texto string, disponibles []VariableEfectiva) (string, error) {
	valores := make(map[string]string, len(disponibles))
	for _, v := range disponibles {
		valores[v.Nombre()] = v.Valor()
	}

	var faltante string
	resultado := patronDeVariable.ReplaceAllStringFunc(texto, func(coincidencia string) string {
		if faltante != "" {
			return coincidencia
		}
		nombre := patronDeVariable.FindStringSubmatch(coincidencia)[1]
		valor, ok := valores[nombre]
		if !ok {
			faltante = nombre
			return coincidencia
		}
		return valor
	})
	if faltante != "" {
		return "", &VariableNoEncontradaError{Nombre: faltante}
	}
	return resultado, nil
}
