package dominio

import "time"

// Regla es qué mira un paso para decidir si se re-ejecuta: código, instrucciones, las variables que ve, tiempo.
// Un conjunto vacío (ninguna activa, sin edad máxima) es válido — DecidirPaso lo trata como "siempre se
// re-ejecuta", nunca como "nunca se re-ejecuta" (ver decidir.go).
type Regla struct {
	miraCodigo        bool
	miraInstrucciones bool
	miraVariables     bool
	edadMaxima        time.Duration
}

func NuevaRegla(miraCodigo, miraInstrucciones, miraVariables bool, edadMaxima time.Duration) (Regla, error) {
	if edadMaxima < 0 {
		return Regla{}, invalido("la edad máxima de una regla no puede ser negativa")
	}
	return Regla{
		miraCodigo: miraCodigo, miraInstrucciones: miraInstrucciones, miraVariables: miraVariables,
		edadMaxima: edadMaxima,
	}, nil
}

func (r Regla) MiraCodigo() bool { return r.miraCodigo }

func (r Regla) MiraInstrucciones() bool { return r.miraInstrucciones }

func (r Regla) MiraVariables() bool { return r.miraVariables }

// EdadMaxima es cero si el paso no caduca.
func (r Regla) EdadMaxima() time.Duration { return r.edadMaxima }
