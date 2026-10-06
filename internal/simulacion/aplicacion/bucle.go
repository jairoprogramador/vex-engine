package aplicacion

import (
	"context"
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// recorrerAmbiente es §2 de SIM-1: recorre los pasos pedidos en este ambiente. Abre un id de simulación propio
// de la llamada y lo cierra siempre, con éxito o al abandonar por SIM-2: los literales del pipeline se repiten
// con distinto valor entre ambientes, y Resolución no distingue el ámbito al declarar por nombre, así que un
// id que sobreviviera a la llamada filtraría en silencio el valor de un ambiente a otro (RD-09 §9).
//
// Devuelve el primer paso con variables que no se pudieron interpolar, o nil si todos interpolan (SIM-2).
func (s *Servicio) recorrerAmbiente(
	ctx context.Context, pipeline dominio.Pipeline, ambiente dominio.Ambiente, pasos []dominio.Paso, meta publicado.Metadatos,
) (faltante *publicado.FaltanteDePaso, err error) {
	id, err := s.d.EspacioTemporal.Nuevo()
	if err != nil {
		return nil, err
	}
	defer func() {
		if errCerrar := s.d.Variables.Cerrar(context.WithoutCancel(ctx), id); errCerrar != nil {
			err = errors.Join(err, errCerrar)
		}
	}()

	ambito, err := dominio.AmbitoDeAmbiente(ambiente.Valor)
	if err != nil {
		return nil, err
	}
	// Las estándar compartidas primero: los literales del pipeline pueden usarlas.
	estandar := s.d.Pipelines.VariablesEstandar()
	compartidas := append(
		comoDeclaradas(estandar, estandarCompartidas(meta, ambiente.Valor), false, ambito),
		variablesDelAmbito(pipeline.Variables, dominio.AmbitoCompartido())...,
	)
	if err := s.d.Variables.Declarar(ctx, id, dominio.AmbitoCompartido(), compartidas); err != nil {
		return nil, err
	}
	propias := variablesDelAmbito(pipeline.Variables, ambito)
	if err := s.d.Variables.Declarar(ctx, id, ambito, propias); err != nil {
		return nil, err
	}

	for _, paso := range pasos {
		if err := s.d.Variables.Declarar(ctx, id, ambito, comoDeclaradas(estandar, estandarDelPaso(paso), true, ambito)); err != nil {
			return nil, err
		}
		variables, err := s.simularPaso(ctx, id, ambito, paso)
		if err != nil {
			return nil, err
		}
		if len(variables) > 0 {
			return &publicado.FaltanteDePaso{Paso: paso.Nombre, Variables: variables}, nil // SIM-2
		}
	}
	return nil, nil
}

func variablesDelAmbito(variables []dominio.VariableDeclarada, ambito dominio.Ambito) []dominio.VariableDeclarada {
	var resultado []dominio.VariableDeclarada
	for _, v := range variables {
		if v.Ambito == ambito {
			resultado = append(resultado, v)
		}
	}
	return resultado
}
