package infraestructura

import (
	"context"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/aplicacion"
)

// Escucha implementa la interfaz publicada del Historial para el evento despliegue registrado, y traduce a
// AlRegistrarseUnDespliegue (LAN-1) en cadenas simples, para que aplicacion/ nunca conozca el lenguaje del
// Historial. La raíz de composición la registra (DEC-05.4): el Historial no sabe quién escucha.
type Escucha struct {
	s *aplicacion.Servicio
}

var _ historialpublicado.EscuchaDespliegueRegistrado = (*Escucha)(nil)

func NuevaEscucha(s *aplicacion.Servicio) *Escucha {
	return &Escucha{s: s}
}

// DespliegueRegistrado no devuelve error: si el aviso no se puede atender, se pierde (IT-05 DEC-05.4). El
// último despliegue del ambiente queda sin lanzar hasta que alguien lo lance a mano o llegue el siguiente.
func (e *Escucha) DespliegueRegistrado(ctx context.Context, evento historialpublicado.DespliegueRegistrado) {
	_ = e.s.AlRegistrarseUnDespliegue(ctx, evento.Despliegue.Ambiente, evento.Despliegue.Id)
}
