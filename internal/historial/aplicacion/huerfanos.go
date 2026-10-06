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

// errDuenoVivo: el dueño del ambiente escribió mientras se decidía que había muerto. Es la prueba de que seguía
// vivo, así que no se toca.
var errDuenoVivo = errors.New("el intento que ocupa el ambiente sigue vivo")

// RegistrarLatido deja constancia de que el proceso del intento sigue vivo. Lo escribe solo ese proceso, así
// que un conflicto solo puede venir de un latido propio repetido y se resuelve volviendo a leer.
func (s *Servicio) RegistrarLatido(ctx context.Context, intento string) error {
	id := dominio.IdIntento(intento)
	return traducir(conReintento(ctx, func() error {
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

// estadoDeVida es lo que se observa de un intento sin desenlace para saber si su proceso sigue vivo.
type estadoDeVida struct {
	terminado bool
	latidos   int
	registros int
	// ultimoLatido es el instante del último latido, según el reloj de su escritor; hayLatido dice si hay alguno.
	ultimoLatido time.Time
	hayLatido    bool
}

func (s *Servicio) vidaDe(ctx context.Context, id dominio.IdIntento) (estadoDeVida, error) {
	intento, err := s.d.Intentos.Intento(ctx, id)
	if err != nil {
		return estadoDeVida{}, err
	}
	latidos, err := s.d.Latidos.Cantidad(ctx, id)
	if err != nil {
		return estadoDeVida{}, err
	}
	ultimo, hay, err := s.d.Latidos.Ultimo(ctx, id)
	if err != nil {
		return estadoDeVida{}, err
	}
	return estadoDeVida{
		terminado: intento.Terminado(), latidos: latidos, registros: len(intento.Registros()),
		ultimoLatido: ultimo, hayLatido: hay,
	}, nil
}

// liberarSiHuerfano dice si el ambiente quedó libre porque el intento que lo ocupaba ya no existe como proceso.
//
// El Historial no puede saber si un intento sin desenlace murió o sigue corriendo en otra máquina, y los
// relojes de dos máquinas no tienen por qué coincidir. Por eso no compara instantes: observa. Mira cuántos
// latidos y registros tiene el intento, espera la ventana de vida, y vuelve a mirar. Si ninguno creció, el
// proceso no escribió nada durante varios latidos y se da por muerto. Un comando largo no engaña: el latido
// lo escribe Ejecución en paralelo, no el comando.
//
// Esperar toda la ventana cuando el dueño vive sería lento, y haría que quien choca con un intento en curso
// tardara 15 s en enterarse. Por eso, si el último latido es más reciente que la ventana, no se espera: se da
// por vivo. Aquí sí se compara con el reloj, pero solo para ahorrar la espera, nunca para declarar muerte: un
// reloj desfasado hace, como mucho, que un huérfano tarde más en recuperarse, y nunca que se libere uno vivo.
//
// Un intento de una versión que no latía parecerá muerto aunque corra; es el precio de poder recuperar un
// ambiente sin intervención, y solo ocurre si dos versiones del motor comparten almacén a la vez.
func (s *Servicio) liberarSiHuerfano(ctx context.Context, id dominio.IdIntento) (bool, error) {
	if s.d.VentanaDeVida <= 0 {
		return false, nil
	}
	antes, err := s.vidaDe(ctx, id)
	if err != nil {
		return false, err
	}
	if antes.terminado {
		return true, nil // lo cerraron mientras tanto: basta con volver a intentar
	}
	if antes.hayLatido && s.d.Reloj.Ahora().Sub(antes.ultimoLatido) < s.d.VentanaDeVida {
		return false, nil // latió hace poco: vive, y no hace falta esperar para comprobarlo
	}
	if err := esperar(ctx, s.d.VentanaDeVida); err != nil {
		return false, err
	}
	despues, err := s.vidaDe(ctx, id)
	if err != nil {
		return false, err
	}
	if despues.terminado {
		return true, nil
	}
	if despues.latidos > antes.latidos || despues.registros > antes.registros {
		return false, nil
	}
	if err := s.darPorInterrumpido(ctx, id, despues.registros); err != nil {
		if errors.Is(err, errDuenoVivo) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// darPorInterrumpido cierra como fallido, con CausaInterrumpido, al intento cuyo proceso murió. Si ni llegó a
// abrirse (la apertura no se pudo escribir) no hay nada que cerrar: se abandona, como en AbandonarIntento.
// registrosVistos es lo que se observó antes de decidir: si el intento tiene más al escribir, estaba vivo.
func (s *Servicio) darPorInterrumpido(ctx context.Context, id dominio.IdIntento, registrosVistos int) error {
	return conReintento(ctx, func() error {
		intento, err := s.d.Intentos.Intento(ctx, id)
		if err != nil {
			return err
		}
		if intento.Terminado() {
			return nil
		}
		if len(intento.Registros()) > registrosVistos {
			return errDuenoVivo
		}
		ahora := s.d.Reloj.Ahora()
		if apertura, abierto := intento.Apertura(); abierto {
			despliegues, err := s.d.Despliegues.DeUnAmbiente(ctx, apertura.Ambiente)
			if err != nil {
				return err
			}
			cierre := dominio.Cierre{Estado: dominio.Fallido, Causa: dominio.CausaInterrumpido}
			if err := intento.Cerrar(cierre, ahora, despliegues); err != nil {
				return err
			}
		} else if err := intento.Abandonar(ahora); err != nil {
			return err
		}
		return s.d.Intentos.Anadir(ctx, intento)
	})
}

// causaQueSePuedePedir valida la causa con que Ejecución cierra un intento. CausaInterrumpido no se puede pedir:
// solo el Historial la escribe, tras comprobar que el dueño murió.
func causaQueSePuedePedir(causa publicado.Causa) (dominio.Causa, error) {
	switch causa {
	case "":
		return "", nil
	case publicado.CausaError:
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
