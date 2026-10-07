package dominio

// Fuente es dónde está el pipeline que se consulta. Es opaca: Catálogo nunca la interpreta.
type Fuente struct{ valor string }

func NuevaFuente(valor string) (Fuente, error) {
	if valor == "" {
		return Fuente{}, invalido("FuenteDelPipeline", valor, "la fuente del pipeline no puede estar vacía")
	}
	return Fuente{valor: valor}, nil
}

func (f Fuente) String() string { return f.valor }

// Commit es el del pipeline que se consulta. Vacío es el de hoy.
type Commit string

func (c Commit) DeHoy() bool { return c == "" }

func (c Commit) String() string { return string(c) }

// Ambiente es un ambiente del pipeline.
type Ambiente struct {
	Nombre      string
	Descripcion string
	Valor       string
}

// Paso es un paso del pipeline.
type Paso struct {
	Nombre     string
	Orden      int
	Compartido bool
}

// Pipeline es lo que Catálogo sabe de un pipeline: sus ambientes y sus pasos, en su orden.
type Pipeline struct {
	Ambientes []Ambiente
	Pasos     []Paso
}

// AmbienteReservable es un ambiente con su estado de reserva.
type AmbienteReservable struct {
	Ambiente
	Reservado bool
}
