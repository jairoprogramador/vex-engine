package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// valoresVisibles resuelve, en el ámbito del paso, las variables que sus comandos marcaron con show. El valor
// se obtiene interpolando ${var.<nombre>}: Resolución no publica ninguna operación que lea valores, a propósito.
// Una variable que no se puede interpolar se omite: enseñar un valor es una cortesía, y no hace fallar el intento.
// Lo que se enseña pasa por la misma redacción que la salida guardada, y se acota (dominio.NuevaVariableVisible):
// un JWT marcado con show no sale en claro, ni un valor enorme entero.
func (s *Servicio) valoresVisibles(
	ctx context.Context, intento string, paso dominio.PasoDeEjecucion, ambito dominio.Ambito,
) []dominio.VariableVisible {
	var visibles []dominio.VariableVisible
	for _, c := range paso.Comandos {
		for _, v := range c.Salidas() {
			if !v.Mostrar() {
				continue
			}
			valor, err := s.d.Variables.Interpolar(ctx, intento, paso.Nombre(), ambito, "${var."+v.Nombre()+"}")
			if err != nil {
				continue
			}
			visibles = append(visibles, dominio.NuevaVariableVisible(v.Nombre(), v.Descripcion(), valor))
		}
	}
	return visibles
}

// acumularVisibles resuelve los visibles de un paso que terminó bien y los suma a los del intento, que van en
// su resultado: un evento de progreso se puede perder, y el resultado no. Un intento enseña como mucho
// dominio.MaximoDeVisibles: las que pasan no se enseñan, ni en el evento ni en el resultado.
func (s *Servicio) acumularVisibles(
	ctx context.Context, c contextoDelIntento, paso dominio.PasoDeEjecucion, ambito dominio.Ambito,
) []dominio.VariableVisible {
	visibles := dominio.CaberEnLosVisibles(len(*c.visibles), s.valoresVisibles(ctx, c.id, paso, ambito))
	*c.visibles = append(*c.visibles, visibles...)
	return visibles
}

func visiblesAPublicado(visibles []dominio.VariableVisible) []publicado.VariableVisible {
	publicados := make([]publicado.VariableVisible, 0, len(visibles))
	for _, v := range visibles {
		publicados = append(publicados, publicado.VariableVisible{Nombre: v.Nombre, Descripcion: v.Descripcion, Valor: v.Valor})
	}
	return publicados
}
