package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// progresoNulo es el Progreso de quien no lo pide: no cuenta nada.
type progresoNulo struct{}

func (progresoNulo) Emitir(context.Context, dominio.EventoDeProgreso) {}

// emitir cuenta un hecho del avance. Sin depender del ctx, como el registro en el Historial: lo que ocurre al
// cancelar es justo lo que más interesa que llegue.
func (s *Servicio) emitir(ctx context.Context, e dominio.EventoDeProgreso) {
	s.d.Progreso.Emitir(context.WithoutCancel(ctx), e)
}
