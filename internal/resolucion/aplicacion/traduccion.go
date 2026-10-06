package aplicacion

import (
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// Traducción entre el modelo y el lenguaje publicado. Es el único sitio donde una variable sale del dominio,
// y por eso donde se deja fuera el valor: publicado.Variable no tiene dónde ponerlo.

func ambitoDeDominio(a publicado.Ambito) (dominio.Ambito, error) {
	if a.Compartido {
		return dominio.AmbitoCompartido(), nil
	}
	return dominio.AmbitoDeAmbiente(a.Ambiente)
}

func ambitoAPublicado(a dominio.Ambito) publicado.Ambito {
	return publicado.Ambito{Compartido: a.EsCompartido(), Ambiente: a.Ambiente()}
}

var origenesPublicados = map[dominio.Origen]publicado.Origen{
	dominio.OrigenDeclarada: publicado.Declarada,
	dominio.OrigenProducida: publicado.Producida,
}

func variablesAPublicado(visibles []dominio.VariableEfectiva) []publicado.Variable {
	variables := make([]publicado.Variable, 0, len(visibles))
	for _, v := range visibles {
		variables = append(variables, publicado.Variable{
			Nombre: v.Nombre(), Origen: origenesPublicados[v.Origen()], Ambito: ambitoAPublicado(v.Ambito()),
		})
	}
	return variables
}

// traducir lleva los errores del dominio al lenguaje publicado, sin perder su mensaje.
func traducir(err error) error {
	if err == nil {
		return nil
	}
	var noEncontrada *dominio.VariableNoEncontradaError
	if errors.As(err, &noEncontrada) {
		return &publicado.VariableNoEncontradaError{Nombre: noEncontrada.Nombre}
	}
	return &errorTraducido{publicado: publicado.ErrRechazado, causa: err}
}

type errorTraducido struct {
	publicado error
	causa     error
}

func (e *errorTraducido) Error() string   { return e.causa.Error() }
func (e *errorTraducido) Unwrap() []error { return []error{e.publicado, e.causa} }
