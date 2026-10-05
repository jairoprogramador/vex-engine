package borde

import (
	"context"
	"errors"
	"fmt"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Lo que el borde deja consultar del Historial nunca lleva el valor de una variable (DEC-04.7): los tipos de
// historialpublicado no tienen dónde ponerlo, y el valor solo sale por la relación reservada hacia Resolución.

// AbandonarIntento comprueba la versión de la petición (DEC-05.6) y da por abandonado un intento sin
// desenlace en el Historial, liberando su ambiente (DEC-07.8).
func (s *Servicio) AbandonarIntento(ctx context.Context, p PeticionDeAbandono) error {
	if err := comprobarVersion(p.Version); err != nil {
		return err
	}
	return s.d.Historial.AbandonarIntento(ctx, p.Intento)
}

// Intento comprueba la versión de la petición (DEC-05.6) y consulta un intento por su identidad.
func (s *Servicio) Intento(
	ctx context.Context, p PeticionDeConsultaDeIntento,
) (historialpublicado.Intento, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return historialpublicado.Intento{}, err
	}
	return s.d.Historial.Intento(ctx, p.Intento)
}

// IntentosDeUnAmbiente comprueba la versión de la petición (DEC-05.6) y consulta los intentos de un ambiente.
func (s *Servicio) IntentosDeUnAmbiente(
	ctx context.Context, p PeticionDeIntentosDeUnAmbiente,
) ([]historialpublicado.Intento, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return nil, err
	}
	return s.d.Historial.IntentosDeUnAmbiente(ctx, p.Ambiente)
}

// DesplieguesDeUnAmbiente comprueba la versión de la petición (DEC-05.6) y consulta los despliegues de un
// ambiente.
func (s *Servicio) DesplieguesDeUnAmbiente(
	ctx context.Context, p PeticionDeDesplieguesDeUnAmbiente,
) ([]historialpublicado.Despliegue, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return nil, err
	}
	return s.d.Historial.DesplieguesDeUnAmbiente(ctx, p.Ambiente)
}

// ErrPeticionInvalida: la petición tiene la forma del lenguaje publicado pero un valor que no es de él.
var ErrPeticionInvalida = errors.New("borde: petición inválida")

// RespuestaDeLogs es la salida de los comandos de un intento: cuál es y de qué ambiente, que si no se pidió
// uno es el último.
type RespuestaDeLogs struct {
	Intento  string
	Ambiente string
	Salidas  []historialpublicado.Salida
}

// Logs comprueba la versión de la petición (DEC-05.6) y consulta la salida de los comandos de un intento: el de
// la petición, o el último que se abrió si no trae ninguno.
func (s *Servicio) Logs(ctx context.Context, p PeticionDeLogs) (RespuestaDeLogs, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return RespuestaDeLogs{}, err
	}
	filtro, err := filtroDeSalidas(p.Resultado)
	if err != nil {
		return RespuestaDeLogs{}, err
	}
	intento, err := s.intentoDeLosLogs(ctx, p.Intento)
	if err != nil {
		return RespuestaDeLogs{}, err
	}
	salidas, err := s.d.Historial.SalidasDeUnIntento(ctx, intento.Id, filtro)
	if err != nil {
		return RespuestaDeLogs{}, err
	}
	return RespuestaDeLogs{Intento: intento.Id, Ambiente: intento.Apertura.Ambiente, Salidas: salidas}, nil
}

// intentoDeLosLogs es el intento pedido, o el último que se abrió si no se pidió ninguno.
func (s *Servicio) intentoDeLosLogs(ctx context.Context, id string) (historialpublicado.Intento, error) {
	if id != "" {
		return s.d.Historial.Intento(ctx, id)
	}
	ultimo, hay, err := s.d.Historial.UltimoIntento(ctx)
	if err != nil {
		return historialpublicado.Intento{}, err
	}
	if !hay {
		return historialpublicado.Intento{}, fmt.Errorf("%w: el historial no tiene ningún intento", historialpublicado.ErrNoExiste)
	}
	return ultimo, nil
}

func filtroDeSalidas(resultado string) (historialpublicado.FiltroDeSalidas, error) {
	switch resultado {
	case "":
		return historialpublicado.TodasLasSalidas, nil
	case "exitoso":
		return historialpublicado.SoloLasExitosas, nil
	case "fallido":
		return historialpublicado.SoloLasFallidas, nil
	}
	return 0, fmt.Errorf("%w: Resultado %q (admitidos: \"exitoso\", \"fallido\" o vacío)", ErrPeticionInvalida, resultado)
}
