package aplicacion

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// declarar interpola y agrega los literales que ya trae Simulación (no se piden a Definición: una petición
// sin intento no toca Historial ni el resto de puertos externos — DEC-04.10).
func (s *Servicio) declarar(
	ctx context.Context, simulacion string, ambito publicado.Ambito, declaradas []publicado.VariableDeclarada,
) error {
	a, err := ambitoDeDominio(ambito)
	if err != nil {
		return traducir(err)
	}
	entradas := make([]dominio.VariableDeclarada, 0, len(declaradas))
	for _, d := range declaradas {
		ambitoDecl, err := ambitoDeDominio(d.Ambito)
		if err != nil {
			return traducir(err)
		}
		entradas = append(entradas, dominio.VariableDeclarada{Nombre: d.Nombre, Ambito: ambitoDecl, Valor: d.Valor})
	}
	if err := declararLiterales(s.invocacion(simulacion), a, entradas); err != nil {
		return fmt.Errorf("resolución: declarar en la simulación: %w", err)
	}
	return nil
}
