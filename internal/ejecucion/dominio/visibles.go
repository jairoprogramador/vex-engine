package dominio

import "unicode/utf8"

const (
	// MaximoDeVisibles es cuántas variables enseña un intento, como mucho: las que pasen no se enseñan.
	MaximoDeVisibles = 32

	// MaximoDeUnValorVisible es cuántos bytes de un valor se enseñan: lo que pase se corta, con una marca.
	MaximoDeUnValorVisible = 4 << 10

	marcaDeValorTruncado = "… [TRUNCADO]"
)

// NuevaVariableVisible es una variable que se enseña a quien invoca, con su valor ya redactado (Redactar) y
// acotado: enseñar un valor es una cortesía hacia quien ejecuta, no un canal para sacar datos del motor.
func NuevaVariableVisible(nombre, descripcion, valor string) VariableVisible {
	return VariableVisible{Nombre: nombre, Descripcion: descripcion, Valor: acotarValorVisible(Redactar(valor))}
}

// acotarValorVisible corta el valor en MaximoDeUnValorVisible bytes, sin partir un carácter, y lo marca.
func acotarValorVisible(valor string) string {
	if len(valor) <= MaximoDeUnValorVisible {
		return valor
	}
	corte := MaximoDeUnValorVisible
	for corte > 0 && !utf8.RuneStart(valor[corte]) {
		corte--
	}
	return valor[:corte] + marcaDeValorTruncado
}

// CaberEnLosVisibles da, de las variables nuevas, las que aún caben en un intento que ya enseña yaVisibles.
func CaberEnLosVisibles(yaVisibles int, nuevas []VariableVisible) []VariableVisible {
	libres := max(MaximoDeVisibles-yaVisibles, 0)
	if len(nuevas) <= libres {
		return nuevas
	}
	return nuevas[:libres]
}
