package dominio

import "time"

// EstadoDePaso es qué pasó con un paso en un intento, según lo que quedó registrado.
type EstadoDePaso string

const (
	PasoEjecutado  EstadoDePaso = "ejecutado"
	PasoPrecargado EstadoDePaso = "precargado" // no se reejecutó: se dio por bueno el de una vez anterior
	PasoFallido    EstadoDePaso = "fallido"
)

// PasoDelDetalle es un paso que se llegó a dar y cómo terminó.
type PasoDelDetalle struct {
	Nombre string
	Estado EstadoDePaso
}

// DetalleDelIntento es una lectura del intento, no un dato nuevo: lo que tardó desde su apertura hasta su último
// registro, y sus pasos en el orden en que se dieron.
type DetalleDelIntento struct {
	Tiempo time.Duration
	Pasos  []PasoDelDetalle
}
