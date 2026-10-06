package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// interpolar sirve a los dos caminos: real (id es un intento) y de simulación (id es lo que el llamador
// decida). No toca Historial ni Definición — solo lee lo que ya está visible en la invocación.
func (s *Servicio) interpolar(ctx context.Context, id string, ambito publicado.Ambito, texto string) (string, error) {
	a, err := ambitoDeDominio(ambito)
	if err != nil {
		return "", traducir(err)
	}
	inv := s.invocacion(id)
	resultado, err := dominio.Interpolar(texto, inv.Visibles(a))
	if err != nil {
		return "", traducir(err)
	}
	return resultado, nil
}
