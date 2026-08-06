// Package cache es el bounded context del caché de re-ejecución: la respuesta a
// «¿esto ya se ejecutó aquí?».
//
// Hasta la spec 10 la palabra «caché» no existía en el modelo. Había cuatro
// «repositorios de status» —uno por regla— con cuatro claves distintas, y no
// había dos iguales: instrucciones y código no llevaban el ambiente, variables
// sí, y tiempo llevaba el ambiente pero no el pipeline. El aislamiento entre
// producción y sandbox dependía de que nadie tocara una lista de exclusiones que
// parecía inocente.
//
// Este paquete introduce el lenguaje —`CacheKey`, `Material`, `Entry`,
// `Provenance`— y con él la regla de vida que faltaba: **el caché es
// desechable**. Se puede borrar entero sin consecuencias. Eso es lo que lo
// separa del ESTADO, que no se borra nunca y que la spec 11 saca de aquí.
package cache

import (
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
)

// Material es todo lo que determina el resultado de ejecutar un paso.
//
// Es explícito y exhaustivo a propósito: obliga a que añadir una dimensión sea
// un cambio de tipo, visible en el compilador, y no un olvido. Así es como el
// ambiente se cayó de tres de las cuatro claves anteriores — nadie tuvo que
// decidir excluirlo.
type Material struct {
	// Subject es la url del proyecto.
	Subject string

	// Pipeline es la url del pipelinecode.
	//
	// Entra por derecho propio, corrigiendo la asimetría de la regla de tiempo,
	// que no lo llevaba: dos pipelines sobre el mismo proyecto compartían TTL.
	Pipeline string

	// Scope es "shared" o el ambiente. OBLIGATORIO y no anulable.
	//
	// No es una mejora de la clave: es su corrección semántica. La huella no
	// responde «¿son iguales las entradas?» sino «¿esto ya se ejecutó AQUÍ?».
	// Un paso tiene efectos sobre un ambiente real, y que dos ambientes tengan
	// entradas idénticas no significa que ejecutar en uno haya dejado algo hecho
	// en el otro.
	//
	// Es UN campo y no un par {Environment, Scope}: serían redundantes mientras
	// coinciden y ambiguos cuando difieran —¿qué significaría `Scope: "shared"`
	// con un `Environment` no vacío?—. `shared` es palabra reservada (spec 04),
	// así que un solo campo no puede colisionar. Hasta la spec 15 vale siempre
	// el ambiente; añadirlo AHORA y no allí es lo que evita reemitir todas las
	// claves.
	Scope string

	// Step es el nombre del paso, sin el prefijo de orden.
	Step string

	// Instructions es la huella de los comandos declarados (inst-v1).
	Instructions fingerprint.Fingerprint

	// Variables es la huella de las variables del paso, menos las volátiles
	// (vars-v1).
	Variables fingerprint.Fingerprint

	// Code es la huella del árbol del proyecto (v1, spec 08).
	Code fingerprint.Fingerprint
}

// El TTL NO entra en el material: el tiempo no es propiedad del contenido. Es
// metadato de expiración de la entrada — ver Entry.

// Validate exige que las siete dimensiones estén presentes.
//
// No es defensa contra el llamador distraído: es la única forma de que «no se
// pudo componer el material» sea distinguible de «se compuso con un hueco». Un
// material con un campo vacío produciría una clave perfectamente válida que
// colisiona con la de cualquier otro material al que le falte el mismo campo, y
// esa colisión se manifestaría como un paso que se salta sin haberse ejecutado
// jamás — el peor fallo posible del motor.
func (m Material) Validate() error {
	campos := []struct {
		nombre string
		valor  string
	}{
		{"subject", m.Subject},
		{"pipeline", m.Pipeline},
		{"scope", m.Scope},
		{"step", m.Step},
	}
	for _, campo := range campos {
		if campo.valor == "" {
			return fmt.Errorf("cache: el material no tiene %s", campo.nombre)
		}
	}

	huellas := []struct {
		nombre string
		valor  fingerprint.Fingerprint
	}{
		{"instructions", m.Instructions},
		{"variables", m.Variables},
		{"code", m.Code},
	}
	for _, huella := range huellas {
		if huella.valor.IsZero() {
			return fmt.Errorf("cache: el material no tiene la huella de %s", huella.nombre)
		}
	}

	return nil
}
