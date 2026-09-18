package dominio

// FormaDeRespuesta es una de tres formas — lista cerrada.
type FormaDeRespuesta string

const (
	ConAtribucion FormaDeRespuesta = "con_atribucion"
	SinReferencia FormaDeRespuesta = "sin_referencia"
	NoSeAtribuye  FormaDeRespuesta = "no_se_atribuye"
)

// Respuesta es una de tres formas, nunca dos a la vez: cada constructor fija su forma y solo puebla los
// campos que le corresponden — no hay forma de construir una atribución junto con no-se-atribuye
// (invariante «nunca atribuye causa a un intento cancelado o sin desenlace»).
type Respuesta struct {
	forma      FormaDeRespuesta
	atribucion Atribucion
	sustento   Sustento
}

func (r Respuesta) Forma() FormaDeRespuesta { return r.forma }

// NuevaRespuestaConAtribucion es ES-1..ES-5: la atribución puede tener de 0 a 3 candidatos — 0 es ES-5, un
// hecho, no un error.
func NuevaRespuestaConAtribucion(atribucion Atribucion, sustento Sustento) Respuesta {
	return Respuesta{forma: ConAtribucion, atribucion: atribucion, sustento: sustento}
}

// RespuestaSinReferencia es ES-6: no existe ningún despliegue contra el que comparar.
func RespuestaSinReferencia() Respuesta { return Respuesta{forma: SinReferencia} }

// RespuestaNoSeAtribuye es ES-7: el intento está cancelado o sin desenlace.
func RespuestaNoSeAtribuye() Respuesta { return Respuesta{forma: NoSeAtribuye} }

// AtribucionConSustento da la atribución y su sustento, y true solo si Forma() es ConAtribucion.
func (r Respuesta) AtribucionConSustento() (Atribucion, Sustento, bool) {
	if r.forma != ConAtribucion {
		return Atribucion{}, Sustento{}, false
	}
	return r.atribucion, r.sustento, true
}
