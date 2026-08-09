package deployment

import (
	"fmt"
	"strings"
)

// Operation es el step PEDIDO —`deploy`, `supply`—, y es lo que la spec 17 §5.6
// renombra para cerrar C-5.
//
// «Step» significaba dos cosas y los documentos previos se contradecían. Las dos
// frases eran correctas y hablaban de cosas distintas:
//
//	Operación (el step PEDIDO)      determina qué se ejecuta   ENTRA en content_id, es el objeto
//	Steps internos (`01-test`…)     estructura interna         entran como material, NO son nodos
//
// Un despliegue por operación pedida: pedir `deploy` corre cuatro steps y produce
// UN objeto, no cuatro.
//
// Es el nombre sin el prefijo de orden —lo que el usuario escribe en la línea de
// comandos—, mientras que `StepContent.StepID` sí lo lleva. No son el mismo dato
// y no deben unificarse: uno es lo que se pidió, el otro es dónde vive.
type Operation struct {
	name string
}

// NewOperation construye la operación pedida.
//
// El vocabulario NO es cerrado, y eso es una decisión de la spec 10 que sigue
// viva: el motor no deriva nada del nombre de un step, así que pedir `notify`
// es tan legítimo como pedir `deploy`. Lo que se rechaza es lo que no puede ser
// un nombre —vacío, o con espacios que lo harían irreconocible en un mensaje—.
func NewOperation(name string) (Operation, error) {
	if name == "" {
		return Operation{}, fmt.Errorf("deployment: el contenido no dice qué operación se pidió")
	}
	if strings.TrimSpace(name) != name || strings.ContainsAny(name, " \t\n\r") {
		return Operation{}, fmt.Errorf(
			"deployment: %q no es una operación: el nombre no puede llevar espacios", name)
	}
	return Operation{name: name}, nil
}

// String es el nombre de la operación.
func (o Operation) String() string { return o.name }

// IsZero indica que no hay operación.
func (o Operation) IsZero() bool { return o.name == "" }

// Equals es la regla de igualdad del value object.
func (o Operation) Equals(other Operation) bool { return o.name == other.name }
