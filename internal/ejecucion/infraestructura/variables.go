package infraestructura

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	resolucionpublicado "github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// Variables implementa dominio.Variables sobre lo que publica Resolución de Variables para un intento real.
type Variables struct {
	variables resolucionpublicado.ParaEjecucion
}

var _ dominio.Variables = (*Variables)(nil)

func NuevasVariables(v resolucionpublicado.ParaEjecucion) *Variables {
	return &Variables{variables: v}
}

func (v *Variables) DeclararVariablesDeUnPaso(
	ctx context.Context, intento, paso string, ambito dominio.Ambito, fuente, commit string, estandar map[string]string,
) error {
	_, err := v.variables.VariablesDeUnPaso(ctx, intento, paso, ambitoResolucion(ambito), fuente, commit, estandar)
	if err != nil {
		return fmt.Errorf("ejecución: declarar las variables del paso %q: %w", paso, err)
	}
	return nil
}

func (v *Variables) Interpolar(ctx context.Context, intento, paso string, ambito dominio.Ambito, texto string) (string, error) {
	interpolado, err := v.variables.Interpolar(ctx, intento, paso, ambitoResolucion(ambito), texto)
	if err != nil {
		return "", fmt.Errorf("ejecución: interpolar en el paso %q: %w", paso, err)
	}
	return interpolado, nil
}

func (v *Variables) HashDeLasVariables(
	ctx context.Context, intento string, ambito dominio.Ambito, textos []string,
) (dominio.HashDeVariables, error) {
	hash, err := v.variables.HashDeLasVariables(ctx, intento, ambitoResolucion(ambito), textos)
	if err != nil {
		return dominio.HashDeVariables{}, fmt.Errorf("ejecución: el hash de las variables: %w", err)
	}
	return dominio.NuevoHashDeVariables(hash), nil
}

func (v *Variables) RegistrarProducido(ctx context.Context, intento, paso, nombre, valor string, ambito dominio.Ambito) error {
	if err := v.variables.RegistrarProducido(ctx, intento, paso, nombre, valor, ambitoResolucion(ambito)); err != nil {
		return fmt.Errorf("ejecución: registrar lo que produjo el paso %q: %w", paso, err)
	}
	return nil
}

func (v *Variables) NoReejecutado(ctx context.Context, intento, paso string, ambito dominio.Ambito) error {
	if err := v.variables.NoReejecutado(ctx, intento, paso, ambitoResolucion(ambito)); err != nil {
		return fmt.Errorf("ejecución: aportar las variables del paso %q que no se reejecutó: %w", paso, err)
	}
	return nil
}

func ambitoResolucion(a dominio.Ambito) resolucionpublicado.Ambito {
	return resolucionpublicado.Ambito{Compartido: a.EsCompartido(), Ambiente: a.Ambiente()}
}
