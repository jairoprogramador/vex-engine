package aplicacion

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

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
	Salidas      dominio.Salidas
	Latidos      dominio.Latidos
	Reloj        dominio.Reloj
	Identidades  dominio.Identidades

	// VentanaDeVida es cuánto observa quien encuentra el ambiente ocupado si el dueño sigue latiendo. Tiene que
	// ser mayor que el intervalo entre latidos de Ejecución —tres veces, para tolerar un latido tardío—. Cero
	// desactiva la recuperación: un ambiente ocupado lo sigue estando hasta que alguien abandone el intento.
	VentanaDeVida time.Duration
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

const (
	// vueltasAnteConflicto es cuántas veces se vuelve a leer y a decidir si otro escribió en el mismo agregado.
	// Cada contenedor es un escritor, y la secuencia de lanzamientos es una para todos los ambientes: con N
	// escritores a la vez, el último necesita hasta N-1 vueltas.
	vueltasAnteConflicto = 10

	// esperaMaxima acota lo que crece la espera entre vueltas.
	esperaMaxima = 250 * time.Millisecond
)

// esperaBase es la espera tras el primer conflicto; se duplica en cada vuelta hasta esperaMaxima. Es una
// variable para que las pruebas no esperen de verdad.
var esperaBase = 2 * time.Millisecond

// conReintento ejecuta una escritura y, si otro escribió a la vez en el mismo agregado (ErrConflicto), vuelve a
// leer y a decidir. Entre vuelta y vuelta espera un tiempo al azar entre cero y un tope que se duplica, para que
// quienes chocaron no vuelvan a chocar todos juntos. Si no lo consigue, devuelve un error que lo dice y que no es
// un fallo del historial: no se escribió nada.
func conReintento(ctx context.Context, escribir func() error) error {
	for vuelta := 1; ; vuelta++ {
		err := escribir()
		if !errors.Is(err, dominio.ErrConflicto) {
			return err
		}
		if vuelta == vueltasAnteConflicto {
			return fmt.Errorf("%w tras %d vueltas: %w", dominio.ErrEscrituraConcurrente, vuelta, err)
		}
		if err := esperarAnteConflicto(ctx, vuelta); err != nil {
			return err
		}
	}
}

func esperarAnteConflicto(ctx context.Context, vuelta int) error {
	tope := min(esperaBase<<(vuelta-1), esperaMaxima)
	if tope <= 0 {
		return ctx.Err()
	}
	temporizador := time.NewTimer(rand.N(tope))
	defer temporizador.Stop()
	select {
	case <-temporizador.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
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
	case errors.Is(err, dominio.ErrEscrituraConcurrente):
		return &errorTraducido{publicado: publicado.ErrEscrituraConcurrente, causa: err}
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
