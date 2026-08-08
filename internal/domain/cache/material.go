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
	// así que un solo campo no puede colisionar. **Vale siempre el ambiente**,
	// también para un paso que declara `scope: project` (spec 13): la dirección
	// donde vive su registro es otra, pero su huella sigue llevando el ambiente
	// aquí. Lo saca la spec 27, que retira de la huella las cuatro dimensiones de
	// DIRECCIÓN; está desde la 10 porque añadirlo después reemitiría todas las
	// claves.
	Scope string

	// Step es el nombre del paso, sin el prefijo de orden.
	Step string

	// Instructions es la huella de los comandos declarados (inst-v1).
	Instructions fingerprint.Fingerprint

	// Variables es la huella de las variables del paso, menos las volátiles
	// (vars-v1).
	Variables fingerprint.Fingerprint

	// Code es la huella del árbol del proyecto (v1, spec 08), y es el ÚNICO
	// término condicional del material (spec 15 §5.2).
	Code fingerprint.Fingerprint

	// CodeExcluded declara que el código del proyecto NO forma parte de la
	// identidad de este paso: es un step con `state_changed: [pipeline]`, que
	// dice que su trabajo no depende del código de la aplicación —crear un
	// registro de contenedores, por ejemplo—.
	//
	// # Por qué esto NO es una `ck-v2`
	//
	// La regla de composición no cambia ni un byte: siguen siendo ocho líneas en
	// el mismo orden, y la línea del código sigue llevando `Q(Code.String())`.
	// Lo que cambia es el DOMINIO de materiales aceptados, y lo hace de forma
	// INYECTIVA: `Q("")` no era producible antes —un `Code` vacío era un error—
	// y ninguna huella lleva la forma canónica vacía, así que ninguna clave
	// emitida cambia de valor y ninguna clave nueva puede coincidir con una
	// vieja. La spec 27 sustituye `ck-v1` entera por `sf-v1`, donde el término
	// del proyecto es condicional por diseño; esto es esa forma, expresada con
	// la regla que hay hoy.
	//
	// # Por qué un booleano en NEGATIVO
	//
	// Porque su valor cero es el SEGURO. Un `IncludesCode bool` dejaría que
	// olvidarlo produjera una huella sin el código —un step que deja de
	// re-ejecutarse ante un cambio de código, la peor omisión posible— mientras
	// que olvidar éste produce una re-ejecución de más. El vacío del material
	// nunca puede ser accidental: `Validate` exige que el hueco esté DECLARADO,
	// que es la diferencia entre «no lo vigila» y «se compuso con un hueco».
	CodeExcluded bool
}

// El TTL NO entra en el material: el tiempo no es propiedad del contenido. Es
// metadato de expiración de la entrada — ver Entry.

// Validate exige que las siete dimensiones estén presentes — seis siempre, y la
// séptima salvo que su ausencia esté DECLARADA (ver `CodeExcluded`).
//
// No es defensa contra el llamador distraído: es la única forma de que «no se
// pudo componer el material» sea distinguible de «se compuso con un hueco». Un
// material con un campo vacío produciría una clave perfectamente válida que
// colisiona con la de cualquier otro material al que le falte el mismo campo, y
// esa colisión se manifestaría como un paso que se salta sin haberse ejecutado
// jamás — el peor fallo posible del motor.
//
// La excepción declarada no reabre esa puerta, y por eso es de doble sentido: un
// material que excluye el código y a la vez lo trae también es un error. Sin esa
// mitad, la forma canónica de un mismo step dependería de si alguien se acordó
// de limpiar el campo, que es la ambigüedad que aquí se está evitando.
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
	}
	for _, huella := range huellas {
		if huella.valor.IsZero() {
			return fmt.Errorf("cache: el material no tiene la huella de %s", huella.nombre)
		}
	}

	if m.CodeExcluded {
		if !m.Code.IsZero() {
			return fmt.Errorf(
				"cache: el material excluye la huella de code y a la vez la trae")
		}
		return nil
	}
	if m.Code.IsZero() {
		return fmt.Errorf("cache: el material no tiene la huella de code")
	}
	return nil
}
