package infraestructura

import (
	"encoding/json"
	"fmt"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Diagnóstico es el único contexto que decodifica el Contenido de otro (docs/modelo/contextos/
// diagnostico.md, «Factoría: el ACL hacia el Historial construye ejes de un paso y referencia a partir de
// registros»): el Historial nunca lo interpreta (DEC-03.13) y cada contexto interpreta el suyo, pero nada
// prohíbe a un tercero que conoce la forma hacerlo también — es justo lo que este ACL hace. Cada copia es
// deliberadamente más angosta que la del que escribe: solo los campos que Diagnóstico necesita. Solo
// decodifica, nunca codifica — Diagnóstico nunca escribe en el Historial.

// contextoRegistroDeEjecucion es la forma de internal/ejecucion/infraestructura/contenido.go
// (contenidoRegistro), escrita en cada RegistroDePaso de tipo Final o NoReejecucion.
const contextoRegistroDeEjecucion = "ejecucion/registro-v1"

type contenidoRegistroDeEjecucion struct {
	HashDelCodigo       string `json:"hash_del_codigo"`
	HashDeInstrucciones string `json:"hash_de_instrucciones"`
}

func decodificarRecursosDePaso(c historialpublicado.Contenido) (hashDelCodigo, hashDeInstrucciones string, err error) {
	if c.Contexto != contextoRegistroDeEjecucion {
		return "", "", fmt.Errorf(
			"diagnóstico: contenido de contexto %q, se esperaba %q", c.Contexto, contextoRegistroDeEjecucion,
		)
	}
	var cr contenidoRegistroDeEjecucion
	if err := json.Unmarshal(c.Datos, &cr); err != nil {
		return "", "", fmt.Errorf("diagnóstico: decodificar los recursos de un paso: %w", err)
	}
	return cr.HashDelCodigo, cr.HashDeInstrucciones, nil
}

// contextoVariableDeResolucion es la forma de internal/resolucion/infraestructura/contenido.go
// (contenidoVariable). El ámbito no le hace falta a este ACL: la visibilidad de una variable declarada ya
// la decide qué registro se leyó (DEC-06.12), y Historial solo guarda la última de cada nombre por paso e
// intento.
const contextoVariableDeResolucion = "resolucion/variable-v1"

type contenidoVariableDeResolucion struct {
	Hash   string `json:"hash"`
	Origen string `json:"origen"`
}

const origenDeclarada = "declarada"

func decodificarVariable(c historialpublicado.Contenido) (hash, origen string, err error) {
	if c.Contexto != contextoVariableDeResolucion {
		return "", "", fmt.Errorf(
			"diagnóstico: contenido de contexto %q, se esperaba %q", c.Contexto, contextoVariableDeResolucion,
		)
	}
	var cv contenidoVariableDeResolucion
	if err := json.Unmarshal(c.Datos, &cv); err != nil {
		return "", "", fmt.Errorf("diagnóstico: decodificar una variable: %w", err)
	}
	return cv.Hash, cv.Origen, nil
}
