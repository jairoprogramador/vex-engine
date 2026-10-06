package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// hashDeLasVariables resume lo que el paso consume, visible desde ambito: las variables del pipeline de ese
// ámbito, las que produjeron los comandos de pasos anteriores y los metadatos que da quien invoca — pero solo
// las que los textos del paso referencian. Una variable que ningún texto del paso usa no puede cambiar su
// resultado, así que no debe re-ejecutarlo. Las generadas por el motor (project_workdir, step_workdir,
// project_hash…) quedan fuera: describen la corrida, no lo que el pipeline declara — el directorio del
// material es otro en cada intento, y el código lo mira su propia regla. Sin esto, «¿cambiaron las
// variables?» diría que sí en cada intento.
func (s *Servicio) hashDeLasVariables(
	_ context.Context, intento string, ambito publicado.Ambito, textos []string,
) (string, error) {
	a, err := ambitoDeDominio(ambito)
	if err != nil {
		return "", traducir(err)
	}

	generadas := map[string]bool{}
	for _, e := range s.d.Definicion.VariablesEstandar() {
		if !e.Metadato {
			generadas[e.Nombre] = true
		}
	}

	usadas := dominio.NombresUsados(textos)
	var entradas []dominio.VariableEfectiva
	for _, v := range s.invocacion(intento).Visibles(a) {
		if usadas[v.Nombre()] && !generadas[v.Nombre()] {
			entradas = append(entradas, v)
		}
	}
	return dominio.HashDeLasEntradas(entradas).String(), nil
}
