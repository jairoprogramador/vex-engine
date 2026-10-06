package aplicacion

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// registrarProducido es el camino real: primero el hash en Historial, después el valor por la relación
// reservada, y solo si las dos escrituras llegan, se pliega en lo visible del intento. Si RegistrarVariable
// falla, no se intenta GuardarValor. Si GuardarValor falla tras un RegistrarVariable exitoso, el hash queda
// escrito pero la variable no se pliega en memoria — pérdida aceptada (ver RD-05 §9).
func (s *Servicio) registrarProducido(
	ctx context.Context, intento, paso, nombre, valor string, ambito publicado.Ambito,
) error {
	a, err := ambitoDeDominio(ambito)
	if err != nil {
		return traducir(err)
	}
	hash := dominio.CalcularHashDeVariable(valor)
	if err := s.d.Historial.RegistrarVariable(ctx, intento, paso, nombre, hash, dominio.OrigenProducida, a); err != nil {
		return fmt.Errorf("resolución: registrar lo que produjo el paso %q: %w", paso, err)
	}
	if err := s.d.Historial.GuardarValor(ctx, intento, paso, nombre, valor); err != nil {
		return fmt.Errorf("resolución: guardar el valor producido por el paso %q: %w", paso, err)
	}
	variable, err := dominio.NuevaVariableEfectiva(nombre, valor, dominio.OrigenProducida, a)
	if err != nil {
		return traducir(err)
	}
	return traducir(s.invocacion(intento).Producir(variable))
}

// producirEnMemoria es el camino de simulación: nunca toca Historial, solo pliega lo que un comando simulado
// dice que produciría.
func (s *Servicio) producirEnMemoria(simulacion, nombre, valor string, ambito publicado.Ambito) error {
	a, err := ambitoDeDominio(ambito)
	if err != nil {
		return traducir(err)
	}
	variable, err := dominio.NuevaVariableEfectiva(nombre, valor, dominio.OrigenProducida, a)
	if err != nil {
		return traducir(err)
	}
	return traducir(s.invocacion(simulacion).Producir(variable))
}
