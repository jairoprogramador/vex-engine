package dominio

import "context"

// TipoDeProgreso es qué se cuenta del avance de un intento.
type TipoDeProgreso string

const (
	// IntentoIniciado: el intento ya está abierto y su espacio de trabajo listo; va a dar su primer paso. Si el
	// ambiente está ocupado o el espacio no está disponible (EJ-5), el intento no empieza y no se cuenta nada.
	IntentoIniciado TipoDeProgreso = "intento_iniciado"
	// PasoIniciado: un paso empieza a ejecutarse, ya con su registro de comienzo escrito. Un paso que no se
	// reejecuta no empieza: solo termina.
	PasoIniciado TipoDeProgreso = "paso_iniciado"
	// PasoTerminado: un paso terminó, con su registro escrito. Su Estado es "ejecutado" o "precargado" (los de
	// DetalleDelIntento) si salió bien, o el desenlace con que acabó: "fallido" o "cancelado".
	PasoTerminado TipoDeProgreso = "paso_terminado"
	// ComandoTerminado: un comando terminó y su salida ya está en el Historial. Su Estado es "exitoso" o "fallido".
	// Nunca lleva la salida: esa se lee con logs.
	ComandoTerminado TipoDeProgreso = "comando_terminado"
)

// EventoDeProgreso es un dato del avance de un intento: solo nombres y resultados, nunca la salida de un comando
// ni el valor de una variable.
type EventoDeProgreso struct {
	Tipo    TipoDeProgreso
	Intento string
	Paso    string // vacío en IntentoIniciado
	Comando string // solo en ComandoTerminado
	Estado  string // solo en PasoTerminado y ComandoTerminado
}

// Progreso es a dónde cuenta Ejecución cómo avanza un intento, mientras avanza. Quien lo implementa decide qué
// hace con ello (la raíz de composición lo manda a quien invoca); el dominio no sabe adónde va.
//
// Emitir nunca falla ni se detiene a esperar: el progreso es una cortesía, y un destino que no lee no puede
// parar un intento. Los eventos llegan en el orden en que ocurren.
type Progreso interface {
	Emitir(ctx context.Context, evento EventoDeProgreso)
}
