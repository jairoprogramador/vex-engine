package aplicacion

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

func (s *Servicio) cambiaronLasVariables(
	ctx context.Context, intento, paso string, ambito publicado.Ambito,
) (bool, error) {
	a, err := ambitoDeDominio(ambito)
	if err != nil {
		return false, traducir(err)
	}
	inv := s.invocacion(intento)

	ahora := map[string]dominio.HashDeVariable{}
	for _, v := range inv.Visibles(a) {
		ahora[v.Nombre()] = dominio.CalcularHashDeVariable(v.Valor())
	}

	ultimaVez, hay, err := s.d.Historial.UltimaVezDeUnPaso(ctx, paso, a)
	if err != nil {
		return false, fmt.Errorf("resolución: ¿cambiaron las variables del paso %q?: %w", paso, err)
	}
	if !hay {
		return true, nil
	}
	return dominio.Cambiaron(ahora, ultimaVez), nil
}
