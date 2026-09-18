package aplicacion

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// noReejecutado aporta, como producidas, las variables de la última vez que el paso se ejecutó de verdad bajo
// ese ámbito. No escribe nada: esos hashes y valores ya están en Historial de cuando se produjeron de verdad.
func (s *Servicio) noReejecutado(ctx context.Context, intento, paso string, ambito publicado.Ambito) error {
	a, err := ambitoDeDominio(ambito)
	if err != nil {
		return traducir(err)
	}
	inv := s.invocacion(intento)

	valores, hay, err := s.d.Historial.ValoresDeLaUltimaVez(ctx, paso, a)
	if err != nil {
		return fmt.Errorf("resolución: variables de la última vez del paso %q: %w", paso, err)
	}
	if !hay {
		return fmt.Errorf("resolución: el paso %q no tiene una última vez de la que aportar variables", paso)
	}
	for nombre, v := range valores {
		variable, err := dominio.NuevaVariableEfectiva(nombre, v.Valor, dominio.OrigenProducida, v.Ambito)
		if err != nil {
			return traducir(err)
		}
		if err := inv.Producir(variable); err != nil {
			return traducir(err)
		}
	}
	return nil
}
