package infraestructura

import (
	"context"
	"fmt"

	resolucionpublicado "github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
)

// Variables implementa dominio.Variables sobre lo que publica Resolución de Variables para una petición sin
// intento (DEC-04.10).
type Variables struct {
	variables resolucionpublicado.ParaSimulacion
}

var _ dominio.Variables = (*Variables)(nil)

func NuevasVariables(v resolucionpublicado.ParaSimulacion) *Variables {
	return &Variables{variables: v}
}

func (v *Variables) Declarar(ctx context.Context, simulacion string, ambito dominio.Ambito, declaradas []dominio.VariableDeclarada) error {
	entradas := make([]resolucionpublicado.VariableDeclarada, 0, len(declaradas))
	for _, d := range declaradas {
		entradas = append(entradas, resolucionpublicado.VariableDeclarada{
			Nombre: d.Nombre, Ambito: ambitoResolucion(d.Ambito), Valor: d.Valor,
		})
	}
	if err := v.variables.Declarar(ctx, simulacion, ambitoResolucion(ambito), entradas); err != nil {
		return fmt.Errorf("simulación: declarar las variables: %w", err)
	}
	return nil
}

func (v *Variables) Interpolar(ctx context.Context, simulacion string, ambito dominio.Ambito, texto string) (string, error) {
	interpolado, err := v.variables.Interpolar(ctx, simulacion, ambitoResolucion(ambito), texto)
	if err != nil {
		return "", fmt.Errorf("simulación: interpolar: %w", err)
	}
	return interpolado, nil
}

func (v *Variables) RegistrarProducido(ctx context.Context, simulacion, nombre, valor string, ambito dominio.Ambito) error {
	if err := v.variables.RegistrarProducido(ctx, simulacion, nombre, valor, ambitoResolucion(ambito)); err != nil {
		return fmt.Errorf("simulación: registrar la salida simulada %q: %w", nombre, err)
	}
	return nil
}

func (v *Variables) Cerrar(ctx context.Context, simulacion string) error {
	if err := v.variables.Cerrar(ctx, simulacion); err != nil {
		return fmt.Errorf("simulación: cerrar la simulación: %w", err)
	}
	return nil
}

func ambitoResolucion(a dominio.Ambito) resolucionpublicado.Ambito {
	return resolucionpublicado.Ambito{Compartido: a.EsCompartido(), Ambiente: a.Ambiente()}
}
