package aplicacion

import (
	"context"
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
	"github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// AbrirIntento ocupa el ambiente y después abre el intento. En ese orden: si la apertura no se pudiera
// escribir, el ambiente queda ocupado por un intento sin apertura, que se da por abandonado. En el orden
// contrario quedaría en el historial un intento que nunca tuvo su ambiente.
func (s *Servicio) AbrirIntento(ctx context.Context, a publicado.Apertura) (string, error) {
	id, err := s.d.Identidades.NuevoIntento()
	if err != nil {
		return "", err
	}
	apertura := aperturaDeDominio(a)
	instante := s.d.Reloj.Ahora()
	intento := dominio.NuevoIntento(id)
	if err := intento.Abrir(apertura, instante, contenidoDeDominio(a.Contenido)); err != nil {
		return "", traducir(err)
	}

	if err := s.ocupar(ctx, apertura.Ambiente, id, instante); err != nil {
		return "", traducir(err)
	}

	if err := s.d.Intentos.Anadir(ctx, intento); err != nil {
		return "", traducir(fmt.Errorf("el ambiente %q quedó ocupado por el intento %s sin apertura: %w",
			apertura.Ambiente, id, err))
	}
	return string(id), nil
}

func (s *Servicio) RegistrarComienzo(ctx context.Context, intento, paso string, contenido publicado.Contenido) error {
	return s.enIntento(ctx, intento, func(i *dominio.Intento, instante time.Time) error {
		return i.Comenzar(dominio.NombrePaso(paso), instante, contenidoDeDominio(contenido))
	})
}

func (s *Servicio) RegistrarFinal(
	ctx context.Context, intento, paso string, exitoso bool, contenido publicado.Contenido,
) error {
	return s.enIntento(ctx, intento, func(i *dominio.Intento, instante time.Time) error {
		return i.Terminar(dominio.NombrePaso(paso), exitoso, instante, contenidoDeDominio(contenido))
	})
}

func (s *Servicio) RegistrarNoReejecucion(
	ctx context.Context, intento, paso string, evidencia publicado.Evidencia, contenido publicado.Contenido,
) error {
	e := dominio.Evidencia{Intento: dominio.IdIntento(evidencia.Intento), Paso: dominio.NombrePaso(evidencia.Paso)}
	apuntado, err := s.d.Intentos.Intento(ctx, e.Intento)
	if err != nil {
		return traducir(err)
	}
	return s.enIntento(ctx, intento, func(i *dominio.Intento, instante time.Time) error {
		return i.NoReejecutar(dominio.NombrePaso(paso), e, apuntado, instante, contenidoDeDominio(contenido))
	})
}

func (s *Servicio) RegistrarVariable(
	ctx context.Context, intento, paso, nombre string, contenido publicado.Contenido,
) error {
	return s.enIntento(ctx, intento, func(i *dominio.Intento, instante time.Time) error {
		return i.RegistrarVariable(
			dominio.NombrePaso(paso), dominio.NombreVariable(nombre), instante, contenidoDeDominio(contenido))
	})
}

// RegistrarSalida guarda la salida de un comando en la secuencia de salidas del intento, aparte de sus
// registros. El intento tiene que existir: una salida sin intento no se podría consultar.
func (s *Servicio) RegistrarSalida(
	ctx context.Context, intento, paso, comando string, exitoso bool, texto string,
) error {
	salida, err := dominio.NuevaSalida(dominio.NombrePaso(paso), comando, exitoso, texto, s.d.Reloj.Ahora())
	if err != nil {
		return traducir(err)
	}
	id := dominio.IdIntento(intento)
	return traducir(conReintento(ctx, func() error {
		if _, err := s.leerIntento(ctx, id); err != nil {
			return err
		}
		previas, err := s.d.Salidas.DeUnIntento(ctx, id)
		if err != nil {
			return err
		}
		return s.d.Salidas.Anadir(ctx, id, len(previas), salida)
	}))
}

// GuardarValor es de la relación reservada.
func (s *Servicio) GuardarValor(ctx context.Context, intento, paso, nombre, valor string) error {
	return s.enIntento(ctx, intento, func(i *dominio.Intento, instante time.Time) error {
		return i.GuardarValor(dominio.NombrePaso(paso), dominio.NombreVariable(nombre), valor, instante)
	})
}

// CerrarIntento escribe el cierre, después el despliegue si el intento llega a él, y después lo anuncia. El
// cierre y el despliegue son dos agregados y dos escrituras. Si la segunda no llega, repetir el mismo cierre
// no escribe otro y completa el despliegue.
func (s *Servicio) CerrarIntento(
	ctx context.Context, id string, estado publicado.Estado, causa publicado.Causa, destino string,
) (publicado.Despliegue, bool, error) {
	causaDeDominio, err := causaQueSePuedePedir(causa)
	if err != nil {
		return publicado.Despliegue{}, false, traducir(err)
	}
	cierre := dominio.Cierre{
		Estado: estadoDeDominio(estado), Causa: causaDeDominio, Destino: dominio.IdDespliegue(destino),
	}
	var intento *dominio.Intento
	err = conReintento(ctx, func() error {
		var err error
		if intento, err = s.leerIntento(ctx, dominio.IdIntento(id)); err != nil {
			return err
		}
		if previo, cerrado := intento.Cierre(); cerrado && previo == cierre {
			return nil
		}
		var despliegues *dominio.DesplieguesDeUnAmbiente
		if a, abierto := intento.Apertura(); abierto {
			if despliegues, err = s.d.Despliegues.DeUnAmbiente(ctx, a.Ambiente); err != nil {
				return err
			}
		}
		if err := intento.Cerrar(cierre, s.d.Reloj.Ahora(), despliegues); err != nil {
			return err
		}
		return s.d.Intentos.Anadir(ctx, intento)
	})
	if err != nil {
		return publicado.Despliegue{}, false, traducir(err)
	}
	return s.desplegar(ctx, intento)
}

func (s *Servicio) desplegar(ctx context.Context, intento *dominio.Intento) (publicado.Despliegue, bool, error) {
	nuevo, err := s.d.Identidades.NuevoDespliegue()
	if err != nil {
		return publicado.Despliegue{}, false, err
	}
	a, _ := intento.Apertura()
	var (
		despliegue dominio.Despliegue
		hay        bool
		escrito    bool
	)
	err = conReintento(ctx, func() error {
		despliegues, err := s.d.Despliegues.DeUnAmbiente(ctx, a.Ambiente)
		if err != nil {
			return err
		}
		if despliegue, hay, err = intento.Desplegar(despliegues, nuevo, s.d.Reloj.Ahora()); err != nil || !hay {
			return err
		}
		escrito = len(despliegues.Nuevos()) > 0
		return s.d.Despliegues.Anadir(ctx, despliegues)
	})
	if err != nil {
		return publicado.Despliegue{}, false, traducir(err)
	}
	if !hay {
		return publicado.Despliegue{}, false, nil
	}
	resultado := despliegueAPublicado(despliegue)
	if escrito {
		for _, escucha := range s.escuchas {
			escucha.DespliegueRegistrado(ctx, publicado.DespliegueRegistrado{Despliegue: resultado})
		}
	}
	return resultado, true, nil
}

// AbandonarIntento vale también para un intento sin apertura, siempre que ocupe un ambiente: es el que queda
// si la apertura no se pudo escribir.
func (s *Servicio) AbandonarIntento(ctx context.Context, id string) error {
	return traducir(conReintento(ctx, func() error {
		intento, err := s.d.Intentos.Intento(ctx, dominio.IdIntento(id))
		if err != nil {
			return err
		}
		if len(intento.Registros()) == 0 {
			if ocupa, err := s.ocupaUnAmbiente(ctx, intento.Id()); err != nil || !ocupa {
				return noExiste(err, "el intento %s", id)
			}
		}
		if err := intento.Abandonar(s.d.Reloj.Ahora()); err != nil {
			return err
		}
		return s.d.Intentos.Anadir(ctx, intento)
	}))
}

func (s *Servicio) RegistrarLanzamiento(
	ctx context.Context, ambiente, despliegue string, contenido publicado.Contenido,
) (publicado.Lanzamiento, error) {
	id, err := s.d.Identidades.NuevoLanzamiento()
	if err != nil {
		return publicado.Lanzamiento{}, err
	}
	var lanzamiento dominio.Lanzamiento
	err = conReintento(ctx, func() error {
		lanzamientos, err := s.d.Lanzamientos.Todos(ctx)
		if err != nil {
			return err
		}
		despliegues, err := s.d.Despliegues.DeUnAmbiente(ctx, dominio.Ambiente(ambiente))
		if err != nil {
			return err
		}
		lanzamiento, err = lanzamientos.Lanzar(
			id, despliegues, dominio.IdDespliegue(despliegue), s.d.Reloj.Ahora(), contenidoDeDominio(contenido))
		if err != nil {
			return err
		}
		return s.d.Lanzamientos.Anadir(ctx, lanzamientos)
	})
	if err != nil {
		return publicado.Lanzamiento{}, traducir(err)
	}
	return lanzamientoAPublicado(lanzamiento), nil
}

func (s *Servicio) RegistrarReserva(ctx context.Context, ambiente string, reservado bool) error {
	return traducir(conReintento(ctx, func() error {
		reservas, err := s.d.Reservas.DeUnAmbiente(ctx, dominio.Ambiente(ambiente))
		if err != nil {
			return err
		}
		if err := reservas.Reservar(reservado, s.d.Reloj.Ahora()); err != nil {
			return err
		}
		return s.d.Reservas.Anadir(ctx, reservas)
	}))
}

// enIntento añade un registro a un intento que existe.
func (s *Servicio) enIntento(
	ctx context.Context, id string, registrar func(*dominio.Intento, time.Time) error,
) error {
	return traducir(conReintento(ctx, func() error {
		intento, err := s.leerIntento(ctx, dominio.IdIntento(id))
		if err != nil {
			return err
		}
		if err := registrar(intento, s.d.Reloj.Ahora()); err != nil {
			return err
		}
		return s.d.Intentos.Anadir(ctx, intento)
	}))
}

// leerIntento lee un intento que tiene registros.
func (s *Servicio) leerIntento(ctx context.Context, id dominio.IdIntento) (*dominio.Intento, error) {
	intento, err := s.d.Intentos.Intento(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(intento.Registros()) == 0 {
		return nil, noExiste(nil, "el intento %s", id)
	}
	return intento, nil
}

func (s *Servicio) ocupaUnAmbiente(ctx context.Context, id dominio.IdIntento) (bool, error) {
	ocupaciones, err := s.d.Ocupaciones.Recorrer(ctx)
	if err != nil {
		return false, err
	}
	for _, ocupacion := range ocupaciones {
		if ultima, ok := ocupacion.Ultima(); ok && ultima.Intento == id {
			return true, nil
		}
	}
	return false, nil
}

func noExiste(err error, formato string, args ...any) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: "+formato, append([]any{dominio.ErrNoExiste}, args...)...)
}
