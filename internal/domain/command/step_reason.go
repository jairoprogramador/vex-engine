package command

// StepReason es el vocabulario CERRADO de POR QUÉ un step terminó como terminó
// (spec 19 §5.1).
//
// # Es lo que el modelo de la spec 17 no llevaba
//
// `step_finished` sabía decir QUÉ pasó —`SUCCESS`, `CACHED`, `SKIPPED`— y no
// POR QUÉ. Los motivos existían desde la spec 15, pero como constantes privadas
// del handler que decide, y viajaban por el log: texto libre, descartable por
// diseño. Aquí pasan a ser dato.
//
// # Los ocho desenlaces, y son ocho porque las specs 13 y 15 añadieron dos cada
// una
//
// Seis dicen por qué el step SE EJECUTÓ, uno por qué NO —`up_to_date`— y otro
// por qué ni siquiera se intentó —`no_commands`—. La distinción que más cuesta
// ver es la de los dos últimos que ejecutan: `no_scope` y `no_rules` terminan
// bien y **no escriben registro**, así que un `step_finished` sin evidencia no
// es una anomalía, es lo normal para ellos.
//
// # Y `undetermined` no puede colapsar en `changed`
//
// «No se pudo componer la huella» ejecuta como «el contenido cambió» y NO se
// disfraza de ello (spec 15). Un booleano —«¿se re-ejecutó?»— perdería
// exactamente la distinción entre un caché que funciona y uno que está roto, que
// es la que un `vex stats` necesita para ser útil.
//
// Cerrado en VALORES, abierto en EXTENSIÓN, como el resto del vocabulario del
// registro: texto libre no es agregable.
//
// # Por qué vive en `command` y no en `record`
//
// Por lo mismo que `StepStatus` y `CommandStatus`, y con el argumento de la
// spec 17 aplicado al revés: aquéllos están aquí y el registro los transporta
// sin redefinirlos, porque inventar un vocabulario paralelo habría dejado el
// original muerto y añadido una traducción que envejece. Éste es el mismo tipo
// de dato —algo que el MOTOR calcula sobre un step— y tiene el mismo dueño.
//
// Y hay una razón de dirección que lo hace obligatorio: `record` importa
// `deployment`, que importa `step`, así que `step` —donde nacen estos motivos—
// no puede importar `record`. `command` es el paquete que las tres cadenas y el
// registro comparten, y es donde el vocabulario común tiene que estar.
type StepReason string

const (
	// ReasonNone es la ausencia de motivo, y es LEGÍTIMA: un step cuyo `before`
	// falla nunca llegó a que nadie decidiera nada sobre él. Decir «no consta» es
	// un hecho; inventarle un motivo sería una conclusión.
	ReasonNone StepReason = ""

	// ReasonNoCommands es el step cuyo `commands.yaml` no declara ningún comando
	// (spec 04 §5.3). Es el único valor de `step.SkipReason`, y el único motivo
	// que acompaña a un estado `SKIPPED`.
	ReasonNoCommands StepReason = "no_commands"

	// ReasonNoScope es el step sin `config.yaml` (spec 13 §5.3): se ejecuta
	// siempre porque no hay dónde recordarlo.
	ReasonNoScope StepReason = "no_scope"

	// ReasonNoRules es el step con ámbito y sin `rules` (spec 15 §5.5): hay dónde
	// recordarse y no hay ninguna afirmación que guardar.
	ReasonNoRules StepReason = "no_rules"

	// ReasonNoRecord es la primera vez: no consta que este step se haya ejecutado
	// bajo esta clave de posición.
	ReasonNoRecord StepReason = "no_record"

	// ReasonChanged es la regla `state_changed`: la huella del step difiere de la
	// del último registro.
	ReasonChanged StepReason = "changed"

	// ReasonExpired es la regla `max_age`: el último registro sigue valiendo pero
	// ha caducado.
	ReasonExpired StepReason = "expired"

	// ReasonUndetermined es el fail-open de la spec 09 §5.3: no se pudo componer
	// la huella, así que se ejecuta. Ver arriba: no es `changed`.
	ReasonUndetermined StepReason = "undetermined"

	// ReasonUpToDate es el step que REVIVIÓ: ninguna regla de las que declara se
	// cumplió, así que no ejecutó ni un comando. Es el motivo que acompaña a
	// `from_cache: true`, y sin él ese booleano no distingue un caché que funciona
	// de una configuración que revive basura.
	ReasonUpToDate StepReason = "up_to_date"
)

// String es la forma externa del motivo, la misma que se escribe en el archivo.
func (r StepReason) String() string { return string(r) }

// IsKnown dice si el motivo pertenece al vocabulario de este motor.
//
// A diferencia de `EventType.IsKnown` —que existe para el LECTOR— ésta se usa
// también al validar la carga: un motivo que este motor no sabe emitir es un
// error DEL EMISOR, no un hecho degradado. Un lector más viejo que se encuentre
// uno nuevo lo conserva tal cual, como con `ErrorClass`.
func (r StepReason) IsKnown() bool {
	switch r {
	case ReasonNone, ReasonNoCommands, ReasonNoScope, ReasonNoRules,
		ReasonNoRecord, ReasonChanged, ReasonExpired, ReasonUndetermined,
		ReasonUpToDate:
		return true
	default:
		return false
	}
}
