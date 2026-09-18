package publicado

import "context"

// PeticionDeDiagnostico es lo que se pregunta: un Intento o un Lanzamiento (nunca los dos), en un
// Ambiente, y opcionalmente una Referencia elegida a mano (DEC-06.6).
type PeticionDeDiagnostico struct {
	Intento     string
	Lanzamiento string
	Ambiente    string
	Referencia  string
}

// ParaBorde es lo que Diagnóstico ofrece al borde del motor (RD-10 lo conecta).
type ParaBorde interface {
	// PreguntarLaCausa es la única operación publicada del core: dado un fallo, dónde está la causa.
	PreguntarLaCausa(ctx context.Context, p PeticionDeDiagnostico) (Respuesta, error)
}
