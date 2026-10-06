package infraestructura

import (
	"encoding/json"
	"fmt"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

// contextoVariable identifica, ante Historial, el contenido de una variable de Resolución (IT-04 DEC-04.7:
// Historial nunca lo interpreta, solo lo guarda). Lleva el hash, el origen y el ámbito con el que se produjo
// — nunca el valor: el valor solo va por la relación reservada.
const contextoVariable = "resolucion/variable-v1"

type contenidoVariable struct {
	Hash       string `json:"hash"`
	Origen     string `json:"origen"`
	Compartido bool   `json:"compartido"`
	Ambiente   string `json:"ambiente,omitempty"`
}

func codificarContenido(
	hash dominio.HashDeVariable, origen dominio.Origen, ambito dominio.Ambito,
) (historialpublicado.Contenido, error) {
	datos, err := json.Marshal(contenidoVariable{
		Hash: hash.String(), Origen: origen.String(),
		Compartido: ambito.EsCompartido(), Ambiente: ambito.Ambiente(),
	})
	if err != nil {
		return historialpublicado.Contenido{}, fmt.Errorf("resolución: codificar el contenido de una variable: %w", err)
	}
	return historialpublicado.Contenido{Contexto: contextoVariable, Datos: datos}, nil
}

func decodificarContenido(c historialpublicado.Contenido) (dominio.HashDeVariable, dominio.Ambito, error) {
	if c.Contexto != contextoVariable {
		return dominio.HashDeVariable{}, dominio.Ambito{},
			fmt.Errorf("resolución: contenido de contexto %q, se esperaba %q", c.Contexto, contextoVariable)
	}
	var cv contenidoVariable
	if err := json.Unmarshal(c.Datos, &cv); err != nil {
		return dominio.HashDeVariable{}, dominio.Ambito{},
			fmt.Errorf("resolución: decodificar el contenido de una variable: %w", err)
	}
	hash, err := dominio.NuevoHashDeVariable(cv.Hash)
	if err != nil {
		return dominio.HashDeVariable{}, dominio.Ambito{}, err
	}
	if cv.Compartido {
		return hash, dominio.AmbitoCompartido(), nil
	}
	ambito, err := dominio.AmbitoDeAmbiente(cv.Ambiente)
	if err != nil {
		return dominio.HashDeVariable{}, dominio.Ambito{}, err
	}
	return hash, ambito, nil
}
