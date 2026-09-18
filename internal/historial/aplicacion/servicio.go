package aplicacion

import (
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
	"github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	"github.com/jairoprogramador/vex-engine/internal/historial/reservado"
)

// Dependencias son los puertos del dominio que conecta la raíz de composición.
type Dependencias struct {
	Intentos     dominio.Intentos
	Despliegues  dominio.Despliegues
	Ocupaciones  dominio.Ocupaciones
	Lanzamientos dominio.Lanzamientos
	Reservas     dominio.Reservas
	Reloj        dominio.Reloj
	Identidades  dominio.Identidades
}

// Servicio atiende todo lo que el Historial publica y la relación reservada. Cada escritura lee su agregado,
// le pide el registro nuevo, que lo comprueba contra los que tiene, y lo añade. Si alguien añadió otro
// mientras tanto, vuelve a leer y a decidir.
type Servicio struct {
	d        Dependencias
	escuchas []publicado.EscuchaDespliegueRegistrado
}

var (
	_ publicado.ParaEjecucion   = (*Servicio)(nil)
	_ publicado.ParaResolucion  = (*Servicio)(nil)
	_ publicado.ParaLanzamiento = (*Servicio)(nil)
	_ publicado.ParaDiagnostico = (*Servicio)(nil)
	_ publicado.ParaBorde       = (*Servicio)(nil)
	_ reservado.Valores         = (*Servicio)(nil)
)

func NuevoServicio(d Dependencias) *Servicio {
	return &Servicio{d: d}
}

// Escuchar registra a quien escucha despliegue registrado.
func (s *Servicio) Escuchar(escucha publicado.EscuchaDespliegueRegistrado) {
	s.escuchas = append(s.escuchas, escucha)
}

// vueltasAnteConflicto es cuántas veces se vuelve a leer y a decidir si otro escribió en el mismo agregado.
const vueltasAnteConflicto = 3

func conReintento(escribir func() error) error {
	for vuelta := 1; ; vuelta++ {
		err := escribir()
		if !errors.Is(err, dominio.ErrConflicto) || vuelta == vueltasAnteConflicto {
			return err
		}
	}
}

// traducir lleva los errores del dominio a los del lenguaje publicado, sin perder su mensaje.
func traducir(err error) error {
	if err == nil {
		return nil
	}
	var ocupado *dominio.AmbienteOcupadoError
	if errors.As(err, &ocupado) {
		return &publicado.AmbienteOcupadoError{Ambiente: string(ocupado.Ambiente), Intento: string(ocupado.Intento)}
	}
	switch {
	case errors.Is(err, dominio.ErrRechazado):
		return &errorTraducido{publicado: publicado.ErrRechazado, causa: err}
	case errors.Is(err, dominio.ErrNoExiste):
		return &errorTraducido{publicado: publicado.ErrNoExiste, causa: err}
	}
	return err
}

type errorTraducido struct {
	publicado error
	causa     error
}

func (e *errorTraducido) Error() string   { return e.causa.Error() }
func (e *errorTraducido) Unwrap() []error { return []error{e.publicado, e.causa} }
