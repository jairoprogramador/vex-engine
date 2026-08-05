package status

// Evidence es lo que una regla OBSERVÓ, separado de lo que concluyó.
//
// Existe porque hasta la spec 09 observar y persistir eran el mismo acto: la
// regla comparaba y, si difería, escribía la huella nueva dentro de `Evaluate`
// —antes de que el step ejecutara un solo comando—. Una muerte dura entre esos
// dos momentos dejaba escrito «sin cambios» para un step que nunca terminó, y la
// corrida siguiente lo saltaba: el peor fallo posible del motor, omitir un
// despliegue creyendo que ya ocurrió.
//
// Devolver la observación en vez de escribirla es lo que permite que OTRO decida
// cuándo persistirla. Ese otro es `StepExecutable`, y el cuándo es «después del
// éxito» (spec 09 §5.2).
//
// DECISIÓN EXPLÍCITA de la spec 09 §5.1, tomada y no omitida: los dos valores son
// `string` crudos, no `fingerprint.Fingerprint`. Desde la spec 08 solo la huella
// del código es un value object versionado; las de instrucciones, variables y
// tiempo siguen siendo cadenas sin tipo ni especificación. Tipar `Evidence`
// obligaría a especificar tres reglas de huella más, y eso es trabajo de la spec
// 10. Lo que sí se sostiene es que la cadena canónica del código viaja ENTERA
// —con su prefijo `v1:`—, porque es la que se compara y la que se persiste: una
// huella sin versión es indistinguible de una huella de otra versión, que es
// justo lo que la spec 08 §5.3 existe para impedir.
type Evidence struct {
	// RuleName identifica a la regla que observó, y es lo que le dice al
	// `StatusWriter` en qué almacén va la observación.
	RuleName string

	// Current es lo observado AHORA. Vacío significa que la regla no llegó a
	// observar nada, y entonces la evidencia no se persiste.
	Current string

	// Previous es lo leído del estado anterior. Vacío puede ser «no había» o
	// «no se pudo leer»; quién de los dos lo dice la `Decision`, no este campo.
	Previous string

	// Changed solo es significativo cuando la decisión es `Run` o `Skip`. Con
	// `Undetermined` queda en false por no afirmar lo que no se observó: el
	// hecho es «no se sabe», y vive en la decisión.
	Changed bool
}

// NewEvidence construye la observación completa de una regla. `changed` se pasa
// explícito y no se deriva de comparar los dos valores: para `TimeRule` «cambió»
// significa «expiró el TTL», que no es una desigualdad de cadenas.
func NewEvidence(ruleName, current, previous string, changed bool) Evidence {
	return Evidence{
		RuleName: ruleName,
		Current:  current,
		Previous: previous,
		Changed:  changed,
	}
}

// NoEvidence es lo que devuelve una regla que no llegó a observar nada: le faltó
// un parámetro, o no pudo calcular la huella actual. No es «no cambió» ni «sí
// cambió» —es que no hay observación—, y por eso nunca se persiste.
func NoEvidence(ruleName string) Evidence {
	return Evidence{RuleName: ruleName}
}

// Persistable dice si esta observación puede escribirse al almacén de estado.
//
// Sin valor actual no hay nada que guardar, y guardar el vacío sería peor que no
// guardar: la corrida siguiente encontraría una huella vacía que coincide con
// otra huella vacía y saltaría el step por una igualdad que nadie observó.
func (e Evidence) Persistable() bool {
	return e.Current != ""
}
