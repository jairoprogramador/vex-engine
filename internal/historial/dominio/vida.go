package dominio

import "time"

// SenalDeVida es lo que se observa de un intento sin desenlace para saber si su proceso sigue vivo: cuántos
// latidos y cuántos registros ha escrito, y cuándo fue el último latido según el reloj de su escritor.
//
// El Historial no puede saber si un intento sin desenlace murió o corre en otra máquina, y los relojes de dos
// máquinas no tienen por qué coincidir. Por eso decide observando, no comparando instantes: si dos señales
// separadas por la ventana de vida no difieren, el proceso no escribió nada durante varios latidos.
type SenalDeVida struct {
	Latidos      int
	Registros    int
	UltimoLatido time.Time
	HayLatido    bool
}

// CrecioDesde dice si el proceso escribió algo —un latido o un registro— desde que se tomó antes.
func (s SenalDeVida) CrecioDesde(antes SenalDeVida) bool {
	return s.Latidos > antes.Latidos || s.Registros > antes.Registros
}

// LatioDentroDe dice si el último latido es más reciente que la ventana. Es solo para ahorrar la espera cuando el
// dueño claramente vive: nunca decide que murió, porque un reloj desfasado haría, como mucho, que un huérfano
// tarde más en recuperarse, y jamás que se libere uno vivo.
func (s SenalDeVida) LatioDentroDe(ventana time.Duration, ahora time.Time) bool {
	return s.HayLatido && ahora.Sub(s.UltimoLatido) < ventana
}
