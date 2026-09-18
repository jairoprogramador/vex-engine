package aplicacion

import (
	"context"
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// recorrerAmbientes es §2 de SIM-1: recorre cada ambiente del pipeline, en su orden.
func (s *Servicio) recorrerAmbientes(ctx context.Context, pipeline dominio.Pipeline) ([]publicado.InformeDeAmbiente, error) {
	compartidas := variablesDelAmbito(pipeline.Variables, dominio.AmbitoCompartido())

	informes := make([]publicado.InformeDeAmbiente, 0, len(pipeline.Ambientes))
	for _, ambiente := range pipeline.Ambientes {
		informe, err := s.recorrerAmbiente(ctx, pipeline, ambiente, compartidas)
		if err != nil {
			return nil, err
		}
		informes = append(informes, informe)
	}
	return informes, nil
}

// recorrerAmbiente abre un id de simulación propio de este ambiente — no uno por llamada a Simular: los
// literales del pipeline se repiten con distinto valor entre ambientes, y Resolución no distingue el ámbito al
// declarar por nombre, así que compartir un id filtraría en silencio el valor de un ambiente a los demás
// (RD-09 §9). Se cierra siempre, con éxito o al abandonarlo por SIM-2.
func (s *Servicio) recorrerAmbiente(
	ctx context.Context, pipeline dominio.Pipeline, ambiente dominio.Ambiente, compartidas []dominio.VariableDeclarada,
) (informe publicado.InformeDeAmbiente, err error) {
	id, err := s.d.EspacioTemporal.Nuevo()
	if err != nil {
		return publicado.InformeDeAmbiente{}, err
	}
	defer func() {
		if errCerrar := s.d.Variables.Cerrar(context.WithoutCancel(ctx), id); errCerrar != nil {
			err = errors.Join(err, errCerrar)
		}
	}()

	ambito, err := dominio.AmbitoDeAmbiente(ambiente.Valor)
	if err != nil {
		return publicado.InformeDeAmbiente{}, err
	}
	if err := s.d.Variables.Declarar(ctx, id, dominio.AmbitoCompartido(), compartidas); err != nil {
		return publicado.InformeDeAmbiente{}, err
	}
	propias := variablesDelAmbito(pipeline.Variables, ambito)
	if err := s.d.Variables.Declarar(ctx, id, ambito, propias); err != nil {
		return publicado.InformeDeAmbiente{}, err
	}

	pasos := make([]publicado.InformeDePaso, 0, len(pipeline.Pasos))
	for _, paso := range pipeline.Pasos {
		informeDelPaso, err := s.simularPaso(ctx, id, ambito, paso)
		if err != nil {
			return publicado.InformeDeAmbiente{}, err
		}
		pasos = append(pasos, informeDelPaso)
		if len(informeDelPaso.Faltante) > 0 {
			break // SIM-2: este ambiente se abandona, sigue el siguiente.
		}
	}
	return publicado.InformeDeAmbiente{Ambiente: ambiente.Nombre, Pasos: pasos}, nil
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
