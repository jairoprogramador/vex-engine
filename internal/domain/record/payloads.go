package record

import (
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// Las cargas útiles del vocabulario cerrado de la spec 17 §5.2.
//
// Los status NO se redefinen aquí: son `command.StepStatus` y
// `command.CommandStatus`, los mismos que el motor ya calcula y que hasta hoy se
// asignaban sin que nadie los leyera (BL-4). Darles un consumidor es parte de lo
// que esta serie de specs viene a terminar; inventar un vocabulario paralelo
// habría dejado el original muerto y añadido una traducción que envejece.
//
// Las duraciones son `time.Duration` y no milisegundos: la unidad es de la
// serialización (`duration_ms`, spec 19), no del dominio.

var (
	_ Payload = AttemptStarted{}
	_ Payload = StepStarted{}
	_ Payload = StepFinished{}
	_ Payload = CommandStarted{}
	_ Payload = CommandFinished{}
	_ Payload = ParameterResolved{}
	_ Payload = ArtifactProduced{}
	_ Payload = StaleCloneUsed{}
	_ Payload = SyncFailed{}
	_ Payload = AttemptFinished{}
)

// AttemptStarted abre el intento. Es el único hecho que lleva el
// `deployment_id`, porque es el que lo acaba de derivar (spec 18).
//
// `actor` y `runner` son CIRCUNSTANCIA y por eso están aquí y no en `Content`:
// quién lanzó el despliegue y sobre qué máquina corrió no cambian qué se
// pretendía hacer. Meterlos en la identidad haría que dos ejecuciones idénticas
// de dos personas distintas fueran objetos distintos, y toda comparación se
// caería.
type AttemptStarted struct {
	Deployment deployment.DeploymentID
	Actor      string
	Runner     string
}

func (p AttemptStarted) Type() EventType { return TypeAttemptStarted }

func (p AttemptStarted) Validate() error {
	if p.Deployment.IsZero() {
		return fmt.Errorf("record: '%s' sin deployment_id", TypeAttemptStarted)
	}
	return nil
}

// StepStarted abre un step.
//
// Lleva `step_fingerprint` y no una `cache_key` opaca, y lleva `scope` como
// CAMPO: las cuatro dimensiones de dirección —proyecto, pipeline, ámbito y
// step— dejan de viajar dentro de un hash y pasan a ser datos del hecho. Un
// consumidor puede filtrar por ámbito sin descomponer nada.
//
// **La huella puede ir VACÍA, y no sólo por fallo.** Desde la spec 15 hay un
// segundo camino y es una declaración: un step que no declara `state_changed`
// —sólo `max_age`, por ejemplo— no compara contenidos, así que el motor no le
// calcula huella. Hay steps que se saltan por VIGENCIA y no por contenido, y el
// modelo no puede tratar la huella como obligatoria.
//
// # Y desde este motor van vacías SIEMPRE, que es un hallazgo de la spec 19
//
// El ámbito y la huella se conocen DENTRO de la cadena de step —el ámbito lo
// anota el handler 03 al leer el `config.yaml`, la huella la compone su
// `decide`— y este hecho se emite ANTES de entrar en ella: es lo que hace que un
// `before` que falla no deje el par abierto (§5.2'). Emitirlo más tarde para
// llenar dos campos costaría la propiedad que el par existe para dar.
//
// Los dos campos se conservan porque son del MODELO y no de este emisor: un
// motor que compusiera la huella antes de abrir el step —lo que la spec 27
// acerca, al mover el material de «acumulado resuelto» a «declarado»— la pondría
// aquí sin cambiar nada. Mientras tanto los dos viajan en `step_finished`, que
// es donde el dato existe, y `Fold` los toma de allí.
type StepStarted struct {
	StepID string
	Scope  state.Scope

	// StepFingerprint es la huella del step en su forma canónica completa, con
	// prefijo (`ck-v1:…` hoy, `sf-v1:…` desde la spec 27). Vacía es legítima.
	StepFingerprint string
}

func (p StepStarted) Type() EventType { return TypeStepStarted }

func (p StepStarted) Validate() error {
	if p.StepID == "" {
		return fmt.Errorf("record: '%s' sin step_id", TypeStepStarted)
	}
	return nil
}

// StepFinished cierra un step, y es el hecho con más carga del vocabulario.
//
// # `from_cache` se lee mejor como «revivido»
//
// Lo que se registra es que el step NO EJECUTÓ porque ninguna regla se cumplió,
// no que un caché acertara. El nombre se conserva; el motivo que lo acompaña
// —qué regla dijo que sí, o que ninguna lo dijo— es de la spec 19, que es quien
// tiene el dato en la mano.
//
// # El hecho es DEL STEP, no de la escritura del registro
//
// Hay tres casos que ejecutan, terminan bien y **no escriben ningún registro**:
// sin `config.yaml` (13 §5.3), sin comandos (04 §5.3) y sin `rules` (15 §5.5),
// los tres colgando de `StepConfig.Remembers()`. Si el hecho colgara del
// almacén, esos steps correrían sin dejar rastro. Una ejecución con steps
// exitosos y CERO registros es normal, y el modelo tiene que poder explicarla.
type StepFinished struct {
	StepID string
	Scope  state.Scope

	// Status es uno de los terminales de `command.StepStatus`:
	// SUCCESS / FAILURE / CACHED / SKIPPED.
	Status command.StepStatus

	Duration time.Duration

	// FromCache dice que el step revivió: no ejecutó ni un comando.
	//
	// Se lee mejor como «revivido», y por sí solo no basta: sin el `Reason` que lo
	// acompaña, un `from_cache: true` no distingue un caché que funciona de una
	// configuración que revive basura durante toda su ventana de vigencia
	// (spec 15 §5.4).
	FromCache bool

	// Reason es POR QUÉ el step terminó como terminó: qué regla dijo que sí, o
	// que ninguna lo dijo. Es lo que la spec 19 añade al modelo, y es extensión y
	// no cambio — `Fold` sigue plegando los hechos anteriores sin tocarse.
	//
	// `ReasonNone` es legítimo: un step cuyo `before` falló nunca llegó a que
	// nadie decidiera nada sobre él.
	Reason command.StepReason

	// StepFingerprint es la huella con la que este step se comparó y bajo la que
	// se registró, en su forma canónica con prefijo. Vacía es legítima por las dos
	// vías de `StepStarted` —fallo al componer el material, o un step que no
	// declara `state_changed`— y además viaja aquí y no allí porque es aquí donde
	// el dato existe (ver `StepStarted`).
	StepFingerprint string

	// Evidence es el registro que estuvo vigente para este step, tanto si revivió
	// como si ejecutó. Ver `EvidenceRef`. El valor cero significa que este step no
	// deja registro, que es lo normal para los tres casos de arriba.
	Evidence EvidenceRef

	// ExitCode es el del comando que hizo fallar el step. Nil cuando no hubo
	// ninguno: un step exitoso, revivido o saltado no tiene código de salida que
	// reportar, y un `0` diría lo contrario.
	ExitCode *int

	// ErrorClass clasifica el fallo. `unknown` es legítimo y preferible a forzar.
	ErrorClass ErrorClass
}

func (p StepFinished) Type() EventType { return TypeStepFinished }

func (p StepFinished) Validate() error {
	if p.StepID == "" {
		return fmt.Errorf("record: '%s' sin step_id", TypeStepFinished)
	}
	if !p.Status.IsTerminal() {
		return fmt.Errorf(
			"record: '%s' con estado '%s', que no es terminal", TypeStepFinished, p.Status)
	}
	if p.Duration < 0 {
		return fmt.Errorf("record: '%s' con duración negativa", TypeStepFinished)
	}
	// Un motivo fuera del vocabulario es un error DEL EMISOR, no un hecho
	// degradado: el consumidor agrega por este campo, y un valor inventado se
	// cuenta aparte para siempre sin que nadie sepa de dónde salió.
	if !p.Reason.IsKnown() {
		return fmt.Errorf(
			"record: '%s' con motivo '%s', que no es del vocabulario", TypeStepFinished, p.Reason)
	}
	return p.Evidence.Validate()
}

// CommandStarted abre un comando dentro de un step.
type CommandStarted struct {
	StepID      string
	CommandName string
}

func (p CommandStarted) Type() EventType { return TypeCommandStarted }

func (p CommandStarted) Validate() error {
	return validateCommandIdentity(TypeCommandStarted, p.StepID, p.CommandName)
}

// CommandFinished cierra un comando.
//
// **Lo que NO lleva: stdout y stderr completos.** Sólo un extracto acotado en
// caso de fallo, y redactado, y eso es de la spec 20 — hasta entonces, ninguno.
// Guardar la salida entera convertiría el registro en un almacén de logs, que es
// justo lo que no es.
type CommandFinished struct {
	StepID      string
	CommandName string

	// Status es `command.CommandSuccess` o `command.CommandFailure`.
	Status command.CommandStatus

	Duration time.Duration

	// ExitCode es el que devolvió el proceso. A diferencia del de un step, aquí
	// es un valor y no un puntero: todo comando que TERMINA tiene uno, y el cero
	// significa lo que significa.
	ExitCode int

	ErrorClass ErrorClass
}

func (p CommandFinished) Type() EventType { return TypeCommandFinished }

func (p CommandFinished) Validate() error {
	if err := validateCommandIdentity(TypeCommandFinished, p.StepID, p.CommandName); err != nil {
		return err
	}
	switch p.Status {
	case command.CommandSuccess, command.CommandFailure:
	default:
		return fmt.Errorf(
			"record: '%s' con estado '%s', que no es de comando", TypeCommandFinished, p.Status)
	}
	if p.Duration < 0 {
		return fmt.Errorf("record: '%s' con duración negativa", TypeCommandFinished)
	}
	return nil
}

func validateCommandIdentity(eventType EventType, stepID, commandName string) error {
	if stepID == "" {
		return fmt.Errorf("record: '%s' sin step_id", eventType)
	}
	if commandName == "" {
		return fmt.Errorf("record: '%s' sin nombre de comando", eventType)
	}
	return nil
}

// ParameterResolved es con qué se resolvió un parámetro, UNO POR PARÁMETRO
// (N-3, la variante `parameters_resolved_digest` se retira).
//
// **El valor no entra: entra su digest.** Es la otra mitad de la regla de la
// spec 14 —en la identidad entra la declaración, nunca el valor— aplicada al
// registro: el valor resuelto se registra como hecho, pero resumido, porque un
// registro que viaja fuera de la organización no puede llevar secretos en claro
// (spec 20).
type ParameterResolved struct {
	Name string

	// Source es de dónde llegó el valor, transportado como `command.Origin`: el
	// enum cuyo orden ES la precedencia. Viaja como DATO, no como conclusión —el
	// consumidor puede derivar de él quién ganó, sin reconstruir el cableado—.
	Source command.Origin

	// Digest es el resumen del valor resuelto. La convención concreta es de la
	// spec 20, que es quien decide qué se redacta y cómo.
	Digest string
}

func (p ParameterResolved) Type() EventType { return TypeParameterResolved }

func (p ParameterResolved) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("record: '%s' sin nombre de parámetro", TypeParameterResolved)
	}
	return nil
}

// ArtifactProduced es lo que un step dejó construido.
//
// La convención del digest está DIFERIDA a propósito (A1/BL-10): el artefacto se
// construye en `package`, es salida del resultado y no entrada del objeto, así
// que fijarla más adelante no invalida nada de lo emitido.
type ArtifactProduced struct {
	StepID string

	// Kind es el `type` del vocabulario de §5.2 —`image`, `jar`…—. Se llama Kind
	// en Go para no chocar con el método `Type()` de la interfaz.
	Kind string

	Digest string
}

func (p ArtifactProduced) Type() EventType { return TypeArtifactProduced }

func (p ArtifactProduced) Validate() error {
	if p.StepID == "" {
		return fmt.Errorf("record: '%s' sin step_id", TypeArtifactProduced)
	}
	if p.Kind == "" {
		return fmt.Errorf("record: '%s' sin tipo de artefacto", TypeArtifactProduced)
	}
	if p.Digest == "" {
		return fmt.Errorf("record: '%s' sin digest", TypeArtifactProduced)
	}
	return nil
}

// StaleCloneUsed es el clon viejo que se usó en vez de fallar (spec 18 §5.4).
//
// Es obligatorio cuando ocurre, y por una razón que no es de auditoría sino de
// identidad: **la identidad se calcula sobre la fuente realmente usada**, así que
// sin este hecho un `content_id` afirmaría que se ejecutó una versión del
// pipeline distinta de la que corrió, y nadie podría saberlo.
type StaleCloneUsed struct {
	// Source es la url de la fuente reutilizada.
	Source string

	// AgeHours es la antigüedad del clon en horas.
	AgeHours float64
}

func (p StaleCloneUsed) Type() EventType { return TypeStaleCloneUsed }

func (p StaleCloneUsed) Validate() error {
	if p.Source == "" {
		return fmt.Errorf("record: '%s' sin fuente", TypeStaleCloneUsed)
	}
	if p.AgeHours < 0 {
		return fmt.Errorf("record: '%s' con antigüedad negativa", TypeStaleCloneUsed)
	}
	return nil
}

// SyncFailed es el empuje del registro que no llegó (spec 21).
type SyncFailed struct {
	// Destination es el sink al que se intentó empujar.
	Destination string

	// Cause es por qué no llegó.
	Cause string
}

func (p SyncFailed) Type() EventType { return TypeSyncFailed }

func (p SyncFailed) Validate() error {
	if p.Destination == "" {
		return fmt.Errorf("record: '%s' sin destino", TypeSyncFailed)
	}
	if p.Cause == "" {
		return fmt.Errorf("record: '%s' sin causa", TypeSyncFailed)
	}
	return nil
}

// AttemptFinished cierra el intento, y su AUSENCIA es lo que justifica el modelo
// entero: sin él, `Fold` sigue produciendo un resultado —`interrupted`, con el
// último step alcanzado— en vez de nada.
//
// Lo emite `CreateExecutionUseCase`, la única capa que ve tanto el éxito como el
// fallo.
type AttemptFinished struct {
	Status AttemptStatus
}

func (p AttemptFinished) Type() EventType { return TypeAttemptFinished }

// Validate rechaza `interrupted` explícitamente: es un estado DERIVADO de la
// ausencia de este hecho, así que emitirlo sería una contradicción —quien está
// vivo para escribirlo no fue interrumpido—.
func (p AttemptFinished) Validate() error {
	switch p.Status {
	case AttemptSucceeded, AttemptFailed, AttemptCancelled:
		return nil
	case AttemptInterrupted:
		return fmt.Errorf(
			"record: '%s' no puede declarar '%s': la interrupción se deriva de su ausencia",
			TypeAttemptFinished, AttemptInterrupted)
	default:
		return fmt.Errorf(
			"record: '%s' con estado '%s', que no es un desenlace", TypeAttemptFinished, p.Status)
	}
}
