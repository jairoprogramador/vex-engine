package dominio

import "time"

// Salida es lo que escribió un comando de un paso al terminar, y si salió bien. Es un registro más del
// historial de un intento, pero vive aparte de los de su secuencia: guardarla no cambia lo que el intento dice
// de sí mismo.
type Salida struct {
	Paso     NombrePaso
	Comando  string
	Exitoso  bool
	Texto    string
	Instante time.Time
}

// NuevaSalida rechaza una salida que no diga de qué paso y de qué comando es: no se podría presentar.
func NuevaSalida(paso NombrePaso, comando string, exitoso bool, texto string, instante time.Time) (Salida, error) {
	if paso == "" {
		return Salida{}, rechazo("una salida necesita el paso al que pertenece")
	}
	if comando == "" {
		return Salida{}, rechazo("una salida necesita el nombre de su comando")
	}
	return Salida{Paso: paso, Comando: comando, Exitoso: exitoso, Texto: texto, Instante: instante}, nil
}
