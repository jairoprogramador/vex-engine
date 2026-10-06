package infraestructura

import (
	"context"
	"fmt"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

// Definicion implementa dominio.Definicion sobre lo que publica Definición de Pipeline.
type Definicion struct {
	pipelines definicionpublicado.ParaResolucion
}

var _ dominio.Definicion = (*Definicion)(nil)

func NuevaDefinicion(pipelines definicionpublicado.ParaResolucion) *Definicion {
	return &Definicion{pipelines: pipelines}
}

func (d *Definicion) VariablesDeclaradas(
	ctx context.Context, fuente, commit string,
) ([]dominio.VariableDeclarada, error) {
	pipeline, err := d.pipelines.DeUnCommit(ctx, fuente, commit)
	if err != nil {
		return nil, fmt.Errorf("resolución: variables declaradas de %s@%s: %w", fuente, commit, err)
	}
	declaradas := make([]dominio.VariableDeclarada, 0, len(pipeline.Variables))
	for _, v := range pipeline.Variables {
		ambito, err := ambitoDeDefinicion(v.Ambito)
		if err != nil {
			return nil, fmt.Errorf("resolución: la variable declarada %q: %w", v.Nombre, err)
		}
		declaradas = append(declaradas, dominio.VariableDeclarada{Nombre: v.Nombre, Ambito: ambito, Valor: v.Valor})
	}
	return declaradas, nil
}

func (d *Definicion) VariablesEstandar() []dominio.VariableEstandar {
	estandar := d.pipelines.VariablesEstandar()
	resultado := make([]dominio.VariableEstandar, 0, len(estandar))
	for _, e := range estandar {
		resultado = append(resultado, dominio.VariableEstandar{Nombre: e.Nombre, Metadato: e.Metadato, DelPaso: e.DelPaso})
	}
	return resultado
}

// ambitoDeDefinicion: en el lenguaje de Definición, el ámbito compartido es el string vacío.
func ambitoDeDefinicion(ambito string) (dominio.Ambito, error) {
	if ambito == "" {
		return dominio.AmbitoCompartido(), nil
	}
	return dominio.AmbitoDeAmbiente(ambito)
}
