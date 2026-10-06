package aplicacion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
	"github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// maxRecuperaciones acota cuántos intentos huérfanos libera una sola apertura: uno basta, y el tope impide
// que un historial raro la mantenga dando vueltas.
const maxRecuperaciones = 3

// RegistrarLatido deja constancia de que el proceso del intento sigue vivo. Lo escribe solo ese proceso, así
// que un conflicto solo puede venir de un latido propio repetido y se resuelve volviendo a leer. Un intento que
// ya terminó no sigue escribiendo, ni siquiera latidos (IT-07 DEC-07.8).
func (s *Servicio) RegistrarLatido(ctx context.Context, intento string) error {
	id := dominio.IdIntento(intento)
	return traducir(conReintento(ctx, func() error {
		dueno, err := s.leerIntento(ctx, id)
		if err != nil {
			return err
		}
		if dueno.Terminado() {
			return fmt.Errorf("%w: el intento %s ya terminó y no late", dominio.ErrRechazado, id)
		}
		previos, err := s.d.Latidos.Cantidad(ctx, id)
		if err != nil {
			return err
		}
		return s.d.Latidos.Anadir(ctx, id, previos, s.d.Reloj.Ahora())
	}))
}

// ocupar ocupa el ambiente con un intento nuevo. Si lo ocupa otro intento, mira si su dueño murió
// (liberarSiHuerfano): de ser así lo cierra y vuelve a intentarlo, para que un proceso caído no bloquee el
// ambiente para siempre. Si el dueño sigue vivo, devuelve *AmbienteOcupadoError como siempre.
func (s *Servicio) ocupar(ctx context.Context, ambiente dominio.Ambiente, id dominio.IdIntento, instante time.Time) error {
	for recuperaciones := 0; ; recuperaciones++ {
		err := conReintento(ctx, func() error {
			ocupacion, err := s.d.Ocupaciones.DeUnAmbiente(ctx, ambiente)
			if err != nil {
				return err
			}
			var anterior *dominio.Intento
			if ultima, ok := ocupacion.Ultima(); ok {
				if anterior, err = s.d.Intentos.Intento(ctx, ultima.Intento); err != nil {
					return err
				}
			}
			if err := ocupacion.Ocupar(id, instante, anterior); err != nil {
				return err
			}
			return s.d.Ocupaciones.Anadir(ctx, ocupacion)
		})

		var ocupado *dominio.AmbienteOcupadoError
		if !errors.As(err, &ocupado) || recuperaciones == maxRecuperaciones {
			return err
		}
		liberado, errLiberar := s.liberarSiHuerfano(ctx, ocupado.Intento)
		if errLiberar != nil {
			if ctx.Err() != nil {
				return errLiberar // se canceló esperando: es una cancelación, no un ambiente ocupado
			}
			return errors.Join(err, errLiberar)
		}
		if !liberado {
			return err
		}
	}
}

// vidaDeUnIntento es lo que se observa de un intento en un instante: si ya terminó y su señal de vida.
type vidaDeUnIntento struct {
	terminado bool
	senal     dominio.SenalDeVida
}

func (s *Servicio) observar(ctx context.Context, id dominio.IdIntento) (vidaDeUnIntento, error) {
	intento, err := s.d.Intentos.Intento(ctx, id)
	if err != nil {
		return vidaDeUnIntento{}, err
	}
	latidos, err := s.d.Latidos.Cantidad(ctx, id)
	if err != nil {
		return vidaDeUnIntento{}, err
	}
	ultimo, hay, err := s.d.Latidos.Ultimo(ctx, id)
	if err != nil {
		return vidaDeUnIntento{}, err
	}
	return vidaDeUnIntento{
		terminado: intento.Terminado(),
		senal: dominio.SenalDeVida{
			Latidos: latidos, Registros: len(intento.Registros()), UltimoLatido: ultimo, HayLatido: hay,
		},
	}, nil
}

// liberarSiHuerfano dice si el ambiente quedó libre porque el intento que lo ocupaba ya no existe como proceso.
// Observa dos veces, separadas por la ventana de vida, y deja que el dominio decida (SenalDeVida, Intento.
// DarPorInterrumpido); aquí solo se orquesta: leer, esperar, leer, escribir. Un comando largo no engaña: el
// latido lo escribe Ejecución en paralelo, no el comando.
//
// Si el último latido es más reciente que la ventana no se espera: se da por vivo, para que quien choca con un
// intento en curso no tarde 15 s en enterarse.
//
// Un intento de una versión que no latía parecerá muerto aunque corra; es el precio de poder recuperar un
// ambiente sin intervención, y solo ocurre si dos versiones del motor comparten almacén a la vez.
func (s *Servicio) liberarSiHuerfano(ctx context.Context, id dominio.IdIntento) (bool, error) {
	if s.d.VentanaDeVida <= 0 {
		return false, nil
	}
	antes, err := s.observar(ctx, id)
	if err != nil {
		return false, err
	}
	if antes.terminado {
		return true, nil // lo cerraron mientras tanto: basta con volver a intentar
	}
	if antes.senal.LatioDentroDe(s.d.VentanaDeVida, s.d.Reloj.Ahora()) {
		return false, nil
	}
	if err := esperar(ctx, s.d.VentanaDeVida); err != nil {
		return false, err
	}
	despues, err := s.observar(ctx, id)
	if err != nil {
		return false, err
	}
	if despues.terminado {
		return true, nil
	}
	if despues.senal.CrecioDesde(antes.senal) {
		return false, nil
	}
	if err := s.darPorInterrumpido(ctx, id, despues.senal.Registros); err != nil {
		if errors.Is(err, dominio.ErrDuenoVivo) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// darPorInterrumpido escribe el cierre por interrupción, protegido por la escritura condicional del intento: si
// el dueño escribió entre la observación y aquí, no se cierra.
func (s *Servicio) darPorInterrumpido(ctx context.Context, id dominio.IdIntento, registrosVistos int) error {
	return conReintento(ctx, func() error {
		intento, err := s.d.Intentos.Intento(ctx, id)
		if err != nil {
			return err
		}
		if intento.Terminado() {
			return nil
		}
		despliegues, err := s.desplieguesParaCerrar(ctx, intento)
		if err != nil {
			return err
		}
		if err := intento.DarPorInterrumpido(registrosVistos, s.d.Reloj.Ahora(), despliegues); err != nil {
			return err
		}
		return s.d.Intentos.Anadir(ctx, intento)
	})
}

// desplieguesParaCerrar son los despliegues del ambiente del intento, que su cierre necesita; ninguno si el
// intento ni llegó a abrirse y se abandona en lugar de cerrarse.
func (s *Servicio) desplieguesParaCerrar(ctx context.Context, intento *dominio.Intento) (*dominio.DesplieguesDeUnAmbiente, error) {
	apertura, abierto := intento.Apertura()
	if !abierto {
		return nil, nil
	}
	return s.d.Despliegues.DeUnAmbiente(ctx, apertura.Ambiente)
}

// causaDeCierreADominio traduce la causa que pide quien cierra. Es la frontera: la de interrumpido no existe
// en CausaDeCierre, y cualquier otra cosa se rechaza.
func causaDeCierreADominio(causa publicado.CausaDeCierre) (dominio.Causa, error) {
	switch causa {
	case "":
		return "", nil
	case publicado.CierrePorError:
		return dominio.CausaError, nil
	}
	return "", fmt.Errorf("%w: la causa %q no se puede pedir al cerrar un intento", dominio.ErrRechazado, causa)
}

func esperar(ctx context.Context, d time.Duration) error {
	temporizador := time.NewTimer(d)
	defer temporizador.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-temporizador.C:
		return nil
	}
}
