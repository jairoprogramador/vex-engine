package dominio

import (
	"regexp/syntax"
	"strings"
)

// maxRepeticion acota cuántas veces se repite una pieza sin límite declarado (*, +, {n,}): lo que importa es
// que la forma se cumpla, no agotar el espacio que la expresión permite. Nunca baja de lo que la propia
// expresión exige como mínimo.
const maxRepeticion = 3

// SalidaSimulada es un valor que cumple la expresión regular de su variable de salida y que nadie produjo
// (simulacion.md, «Value objects»).
type SalidaSimulada string

// FabricarSalidaSimulada es el servicio de dominio: a partir de la expresión regular de una variable de
// salida, sintetiza un valor que la cumple (simulacion.md, «Servicio de dominio»). Recorre el árbol de la
// expresión analizada y construye, para cada nodo, la pieza de texto más simple que lo satisface — el primer
// literal, la primera rama de una alternancia, el mínimo de repeticiones exigido (acotado por maxRepeticion).
func FabricarSalidaSimulada(expresion string) (SalidaSimulada, error) {
	analizada, err := syntax.Parse(expresion, syntax.Perl)
	if err != nil {
		return "", invalido("la expresión regular %q no es correcta: %v", expresion, err)
	}
	var b strings.Builder
	generar(analizada, &b)
	return SalidaSimulada(b.String()), nil
}

func generar(re *syntax.Regexp, b *strings.Builder) {
	switch re.Op {
	case syntax.OpLiteral:
		b.WriteString(string(re.Rune))
	case syntax.OpCharClass:
		b.WriteRune(primeraRunaDeLaClase(re.Rune))
	case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
		b.WriteRune('x')
	case syntax.OpCapture, syntax.OpConcat:
		for _, sub := range re.Sub {
			generar(sub, b)
		}
	case syntax.OpAlternate:
		if len(re.Sub) > 0 {
			generar(re.Sub[0], b)
		}
	case syntax.OpQuest:
		repetir(re.Sub[0], repeticiones(0, 1), b)
	case syntax.OpStar:
		repetir(re.Sub[0], repeticiones(0, -1), b)
	case syntax.OpPlus:
		repetir(re.Sub[0], repeticiones(1, -1), b)
	case syntax.OpRepeat:
		repetir(re.Sub[0], repeticiones(re.Min, re.Max), b)
	}
	// OpEmptyMatch, OpBeginLine, OpEndLine, OpBeginText, OpEndText, OpWordBoundary, OpNoWordBoundary y
	// OpNoMatch no producen texto: son condiciones sobre la posición, no contenido.
}

func repetir(re *syntax.Regexp, n int, b *strings.Builder) {
	for range n {
		generar(re, b)
	}
}

// repeticiones decide cuántas veces repetir una pieza: al menos min, como mucho max si lo declara, acotado a
// maxRepeticion salvo que min ya lo supere — lo que la expresión exige como mínimo nunca se rebaja.
func repeticiones(min, max int) int {
	n := min
	if n < 1 {
		n = 1
	}
	if n < maxRepeticion {
		n = maxRepeticion
	}
	if max >= 0 && n > max {
		n = max
	}
	if n < min {
		n = min
	}
	return n
}

// primeraRunaDeLaClase elige, de las rachas de una clase de caracteres, una runa imprimible en ASCII si hay
// alguna disponible — evita devolver un carácter de control (frecuente en clases negadas como \D o [^0-9],
// cuya primera racha suele empezar en 0) sin cambiar qué expresión se está cumpliendo.
func primeraRunaDeLaClase(rachas []rune) rune {
	for i := 0; i+1 < len(rachas); i += 2 {
		inicio, fin := rachas[i], rachas[i+1]
		if inicio < 'a' && fin >= 'a' {
			inicio = 'a'
		}
		if inicio >= 32 && inicio < 127 {
			return inicio
		}
	}
	if len(rachas) > 0 {
		return rachas[0]
	}
	return 'x'
}
