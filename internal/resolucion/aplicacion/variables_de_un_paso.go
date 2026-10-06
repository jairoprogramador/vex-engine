package aplicacion

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

func (s *Servicio) variablesDeUnPaso(
	ctx context.Context, intento, paso string, ambito publicado.Ambito, fuente, commit string,
	estandar map[string]string,
) ([]publicado.Variable, error) {
	a, err := ambitoDeDominio(ambito)
	if err != nil {
		return nil, traducir(err)
	}
	inv := s.invocacion(intento)

	for _, e := range s.d.Definicion.VariablesEstandar() {
		ambitoEstandar := a
		if !e.DelPaso {
			ambitoEstandar = dominio.AmbitoCompartido()
		}
		variable, err := dominio.NuevaVariableEfectiva(e.Nombre, estandar[e.Nombre], dominio.OrigenDeclarada, ambitoEstandar)
		if err != nil {
			return nil, traducir(err)
		}
		if err := inv.Declarar(variable); err != nil {
			return nil, traducir(err)
		}
	}

	declaradas, err := s.d.Definicion.VariablesDeclaradas(ctx, fuente, commit)
	if err != nil {
		return nil, fmt.Errorf("resolución: variables del paso %q: %w", paso, err)
	}
	if err := declararLiterales(inv, a, declaradas); err != nil {
		return nil, fmt.Errorf("resolución: variables del paso %q: %w", paso, err)
	}

	return variablesAPublicado(inv.Visibles(a)), nil
}

// declararLiterales declara los literales visibles desde a, interpolando cada uno contra lo ya declarado.
//
// El orden de declaradas no es de dependencia (RD-04 §9.16: «el orden pasa a comprobarse donde se usa, no
// donde se declara») — Definición solo garantiza que no hay ciclos ni referencias que no se vean, nunca que
// un literal aparezca en la lista después de los que usa. Por eso se declara por punto fijo: en cada vuelta
// se declaran los que ya interpolan con lo que hay hasta ahora, hasta que no quede ninguno pendiente o una
// vuelta entera no consiga avanzar ninguno (lo que, para un pipeline comprobado, no debería pasar).
func declararLiterales(inv *dominio.VariablesDeUnaInvocacion, a dominio.Ambito, declaradas []dominio.VariableDeclarada) error {
	var pendientes []dominio.VariableDeclarada
	for _, decl := range declaradas {
		if decl.Ambito.Ve(a) {
			pendientes = append(pendientes, decl)
		}
	}

	for len(pendientes) > 0 {
		var siguientes []dominio.VariableDeclarada
		var primerError error
		progreso := false
		for _, decl := range pendientes {
			valor, err := dominio.Interpolar(decl.Valor, inv.Visibles(a))
			if err != nil {
				if primerError == nil {
					primerError = err
				}
				siguientes = append(siguientes, decl)
				continue
			}
			variable, err := dominio.NuevaVariableEfectiva(decl.Nombre, valor, dominio.OrigenDeclarada, decl.Ambito)
			if err != nil {
				return err
			}
			if err := inv.Declarar(variable); err != nil {
				return err
			}
			progreso = true
		}
		if !progreso {
			return fmt.Errorf("%d variable(s) declarada(s) no se pudieron interpolar: %w", len(siguientes), primerError)
		}
		pendientes = siguientes
	}
	return nil
}
