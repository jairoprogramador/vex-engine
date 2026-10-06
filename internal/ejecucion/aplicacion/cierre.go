package aplicacion

import (
	"context"
	"errors"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// abandonar libera el ambiente de un intento que se abrió y no llegó a empezar, porque algo falló antes del primer
// paso (EJ-5: el espacio de trabajo no está disponible). El Historial se consulta primero para reservar el
// ambiente (así dos intentos no se pisan el espacio de trabajo), y por eso un fallo posterior tiene que
// soltarlo: un intento sin desenlace lo dejaría ocupado hasta que alguien lo abandonara a mano. Devuelve la
// causa; si además no se pudo abandonar, lo dice, porque entonces el ambiente sí puede haber quedado ocupado.
func (s *Servicio) abandonar(ctx context.Context, id string, causa error) error {
	// context.WithoutCancel: abandonar es la consecuencia del fallo, no algo que una cancelación deba impedir.
	if err := s.d.Historial.AbandonarIntento(context.WithoutCancel(ctx), id); err != nil {
		return errors.Join(causa, fmt.Errorf("ejecución: el intento %q no llegó a empezar y no se pudo abandonar: el ambiente puede seguir ocupado: %w", id, err))
	}
	return causa
}

// cerrar registra el desenlace del intento y, si llega a despliegue, deja su identidad en el resultado —
// nunca la tiene si el intento se hizo con una copia de trabajo (DEC-10.7) o no terminó exitoso.
func (s *Servicio) cerrar(ctx context.Context, id string, intento *dominio.IntentoEnCurso, destino string) (publicado.Resultado, error) {
	desenlace, hay := intento.Desenlace()
	if !hay {
		return publicado.Resultado{}, fmt.Errorf("ejecución: el intento %q no llegó a un desenlace", id)
	}
	// context.WithoutCancel: un intento cancelado (EJ-3) tiene que poder cerrarse como cancelado — cerrar es la
	// consecuencia de la cancelación, no algo que ella misma deba impedir.
	despliegue, _, err := s.d.Historial.CerrarIntento(context.WithoutCancel(ctx), id, desenlace, destino)
	if err != nil {
		return publicado.Resultado{}, fmt.Errorf("ejecución: cerrar el intento %q: %w", id, err)
	}
	// El detalle es una lectura de cortesía: el intento ya está cerrado, y perder su identidad y su desenlace
	// porque no se pudo releer sería peor que devolverlos sin detalle.
	detalle, err := s.d.Historial.DetalleDelIntento(context.WithoutCancel(ctx), id)
	if err != nil {
		detalle = dominio.DetalleDelIntento{}
	}
	return resultadoAPublicado(id, desenlace, despliegue, detalle), nil
}
