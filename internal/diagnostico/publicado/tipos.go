package publicado

import "time"

// Eje es código, instrucciones o variables — lista cerrada (DEC-01.2).
type Eje string

const (
	Codigo        Eje = "codigo"
	Instrucciones Eje = "instrucciones"
	Variables     Eje = "variables"
)

// RazonDeReferencia dice por qué se eligió una referencia.
type RazonDeReferencia string

const (
	MismoAmbiente       RazonDeReferencia = "mismo_ambiente"
	ElegidaPorElUsuario RazonDeReferencia = "elegida_por_el_usuario"
)

// CambioDeVariable nombra, en un paso, una variable que cambió. Nunca su valor (IT-04 DEC-04.7).
type CambioDeVariable struct {
	Paso   string
	Nombre string
}

// CambioDeEje es un eje que cambió, y en qué pasos.
type CambioDeEje struct {
	Eje   Eje
	Pasos []string
}

// Comparacion es el intento que falla contra una referencia. CantidadDeIntentos solo está presente
// (HayCantidadDeIntentos) para la referencia del mismo ambiente (ES-8).
type Comparacion struct {
	Despliegue                  string
	Ambiente                    string
	Razon                       RazonDeReferencia
	Instante                    time.Time
	PasosComparadosPorEvidencia []string
	CantidadDeIntentos          int
	HayCantidadDeIntentos       bool
}

// Sustento es lo que se sabe de cada cambio: nunca autores ni commits (DEC-07.7).
type Sustento struct {
	InstanteDelIntentoQueFalla   time.Time
	EjesCambiados                []CambioDeEje
	VariablesDeclaradasCambiadas []CambioDeVariable
	VariablesProducidasCambiadas []CambioDeVariable
	PasosComparadosPorEvidencia  []string
	Comparaciones                []Comparacion
}

// FormaDeRespuesta es una de tres formas — lista cerrada.
type FormaDeRespuesta string

const (
	ConAtribucion FormaDeRespuesta = "con_atribucion"
	SinReferencia FormaDeRespuesta = "sin_referencia"
	NoSeAtribuye  FormaDeRespuesta = "no_se_atribuye"
)

// Respuesta es una de tres formas. Atribucion y Sustento solo están poblados cuando Forma es ConAtribucion
// — una atribución vacía (Atribucion == nil) es ES-5, un hecho, no un error. Mensaje solo está poblado
// cuando Forma es SinReferencia (ES-6).
type Respuesta struct {
	Forma      FormaDeRespuesta
	Atribucion []Eje
	Sustento   Sustento
	Mensaje    string
}
