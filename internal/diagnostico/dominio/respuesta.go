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
	mensaje    string
}

// MensajeSinHistorialPrevio es lo que se le dice al usuario en ES-6.
const MensajeSinHistorialPrevio = "no hay historial previo para poder diagnosticar"

func (r Respuesta) Forma() FormaDeRespuesta { return r.forma }

// Mensaje es el texto para el usuario cuando la forma lo requiere (ES-6); vacío en las demás.
func (r Respuesta) Mensaje() string { return r.mensaje }

// NuevaRespuestaConAtribucion es ES-1..ES-5: la atribución puede tener de 0 a 3 candidatos — 0 es ES-5, un
// hecho, no un error.
func NuevaRespuestaConAtribucion(atribucion Atribucion, sustento Sustento) Respuesta {
	return Respuesta{forma: ConAtribucion, atribucion: atribucion, sustento: sustento}
}

// RespuestaSinReferencia es ES-6: no existe ningún despliegue anterior al intento contra el que comparar.
func RespuestaSinReferencia() Respuesta {
	return Respuesta{forma: SinReferencia, mensaje: MensajeSinHistorialPrevio}
}

// RespuestaNoSeAtribuye es ES-7: el intento está cancelado o sin desenlace.
func RespuestaNoSeAtribuye() Respuesta { return Respuesta{forma: NoSeAtribuye} }

// AtribucionConSustento da la atribución y su sustento, y true solo si Forma() es ConAtribucion.
func (r Respuesta) AtribucionConSustento() (Atribucion, Sustento, bool) {
	if r.forma != ConAtribucion {
		return Atribucion{}, Sustento{}, false
	}
	return r.atribucion, r.sustento, true
}
