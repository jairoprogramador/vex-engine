package step

import (
	"context"
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// Motivos por los que un step se ejecuta.
//
// La decisión se reduce a: **leer el último registro de la clave de posición y
// evaluar sobre él las reglas que el step DECLARA**. No hay `Policy`, ni
// `PolicyBuilder`, ni `RuleRegistry`, ni `Decision`, ni `Evidence`, ni cuatro
// repositorios; y desde la spec 15 tampoco hay una política única del motor.
//
// Y no consulta el índice. Ésa es la propiedad que la spec 11 §5.5 compra y que
// el harness comprueba borrándolo: `rm -rf $HOME/.vex/cache` no cambia una sola
// de estas decisiones.
//
// LA DUDA NO NECESITA UN TIPO, pero sí tiene que decirse. La spec 10 se llevó
// `Decision`, `Action`, `Evidence` y `Undetermined`, y la 15 no los reintroduce:
// las reglas se combinan con un OR sobre booleanos. Lo que se conserva es lo que
// era el defecto, que nunca fue el tipo sino el silencio: «no pude componer la
// huella» ejecuta igual que «el contenido cambió», pero NO se disfraza de ello
// —viaja en su propio motivo—.
const (
	reasonNoRecord     = "no consta que este step se haya ejecutado aquí"
	reasonChanged      = "el contenido del step cambió desde la última ejecución"
	reasonExpired      = "el último registro del step ha caducado"
	reasonUndetermined = "no se pudo determinar si ya se ejecutó"
	reasonNoScope      = "el step no declara ámbito en su config.yaml: no hay dónde recordarlo"
	reasonNoRules      = "el step no declara reglas de re-ejecución en su config.yaml: no hay nada que comprobar"
)

// AQUÍ vivía `defaultMaxAge`, y con él el TTL global de 30 días. Lo retira la
// spec 15 §5.7: **un step sin `max_age` no caduca**.
//
// Era una política DEL MOTOR sobre despliegues ajenos, heredada de cuando la
// regla de tiempo la aplicaba sólo a `test` y la spec 10 la extendió a todos por
// unificación de clave. El cambio de comportamiento va en la dirección menos
// habitual —el motor ejecuta MENOS— y conviene decirlo con precisión: hasta aquí
// un `deploy` que llevaba 31 días sin tocarse se re-ejecutaba solo; a partir de
// aquí eso ocurre si el pipelinecode lo pide con `max_age`, porque quien sabe
// cada cuánto conviene revisar un despliegue es quien lo escribió.

// reasonOf traduce la regla que cerró el OR a la frase que el usuario lee.
//
// La traducción vive AQUÍ y no en la regla a propósito (§5.1'): el motivo es del
// step, no de la regla, porque lo que se emite es «este step se re-ejecuta porque
// X» y X es la primera regla que dijo que sí. Una regla que devolviera su propia
// frase estaría hablando con el usuario, que no es su trabajo.
func reasonOf(kind RuleKind) string {
	switch kind {
	case RuleKindStateChanged:
		return reasonChanged
	case RuleKindMaxAge:
		return reasonExpired
	default:
		return reasonUndetermined
	}
}

// StepRunnerHandler hace DOS cosas desde la spec 18: decidir si el step se
// ejecuta y ejecutarlo. La tercera —cargar sus comandos y su configuración— se
// izó a la cadena de pipeline (§5.2'), donde el material se lee una sola vez y
// antes de ejecutar nada.
type StepRunnerHandler struct {
	StepBaseHandler
	loaded  *LoadedPipelinecode
	records state.Records
}

var _ StepHandler = (*StepRunnerHandler)(nil)

func NewStepRunnerHandler(loaded *LoadedPipelinecode, records state.Records) StepHandler {
	return &StepRunnerHandler{
		StepBaseHandler: StepBaseHandler{Next: nil},
		loaded:          loaded,
		records:         records,
	}
}

func (h *StepRunnerHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	material, err := h.loaded.Get(request.StepFullName())
	if err != nil {
		return fmt.Errorf("cargar commands: %w", err)
	}
	commands := material.CommandsCopy()

	// Un step sin comandos no es un éxito: es un skip con razón. La diferencia
	// no es de vocabulario —el step deja de persistir estado de re-ejecución, que
	// es lo que hacía que un `commands.yaml` vacío quedara escrito como «sin
	// cambios» para siempre (spec 04 §5.3, D-A12).
	if len(commands) == 0 {
		request.MarkStepSkipped(SkipReasonNoCommands)
		request.Emit(fmt.Sprintf("%s se salta: no hay comandos para ejecutar (%s)",
			request.StepNameExe(), SkipReasonNoCommands))
		if h.Next != nil {
			return h.Next.Handle(ctx, request)
		}
		return nil
	}

	// La configuración se anota AQUÍ, después de saber que hay algo que ejecutar
	// y antes de decidir si ejecutarlo: trae el ámbito, que dice DÓNDE se consulta
	// y dónde se escribirá (spec 13), y las reglas, que dicen CUÁNDO lo guardado
	// deja de valer (spec 15). Que un `config.yaml` presente declare un `scope` y
	// unas `rules` del vocabulario cerrado ya lo comprobó el validador de la
	// spec 04 antes del primer step.
	//
	// Se LEÍA aquí hasta la spec 18. Ahora llega con el resto del material del
	// step, y el orden de la cadena no cambia: el ámbito y las reglas se ponen en
	// el request justo antes de que la decisión los mire.
	config := material.Config
	request.SetStepConfig(config)
	h.warnIfOnlyExpires(request, config)

	// AQUÍ desapareció el `policyBuilder.Build` que abortaba la ejecución ante
	// un step con nombre desconocido (spec 05 §5.2). No se sustituyó por nada:
	// la decisión sale del CONTENIDO y de la POSICIÓN, no del nombre, así que un
	// `05-notify` ya no necesita que el motor tenga sus comprobaciones cableadas.
	// No hay registro bajo su clave, luego se ejecuta; al terminar bien, deja el
	// suyo; la corrida siguiente lo revive. Es el destino de P1, entregado por
	// eliminación (spec 10 §5.3bis).
	fingerprint, run, reason, err := h.decide(ctx, request, commands)
	if err != nil {
		return err
	}

	if !run {
		request.Emit(fmt.Sprintf("%s ya fue ejecutado y se mantiene sin cambios%s",
			request.StepNameExe(), reason))
		if h.Next != nil {
			return h.Next.Handle(ctx, request)
		}
		return nil
	}

	// La huella queda anotada ANTES de ejecutar, pero sólo se escribe si se llega
	// al final. Anotar no persiste nada: si el step falla —o si el proceso muere
	// a mitad— la anotación muere con la cadena, y la corrida siguiente vuelve a
	// no encontrar registro. Ésa es la ventana que la spec 09 cerró y que esta
	// spec sólo tiene que no reabrir al cambiar QUÉ se escribe.
	if !fingerprint.IsZero() {
		request.RecordStepFingerprint(fingerprint)
	}

	// A partir de aquí hay ejecución real, y por tanto habrá un hecho que
	// registrar si termina bien. El step que revivió salió por el `return` de
	// arriba sin pasar por aquí: revivir no es un hecho nuevo del step.
	request.MarkStepExecuted()

	request.Emit(fmt.Sprintf("Ejecutando %s: %s", request.StepNameExe(), reason))
	for _, cmd := range commands {
		request.AddCommand(cmd)
		if err := request.Execute(); err != nil {
			return err
		}
	}
	request.Emit(fmt.Sprintf("%s ejecutado correctamente", request.StepNameExe()))

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}

// warnIfOnlyExpires emite el aviso de §5.4: reglas de expiración y ninguna de
// invalidación.
//
// Un step así revive un resultado obsoleto durante toda su ventana de vigencia
// aunque su huella haya cambiado de forma evidente. **No es un error y no se
// prohíbe** —nada es implícito, todo se declara, y no existe una comprobación que
// el motor imponga por fuera de lo que el `config.yaml` dice— pero se avisa, para
// que sea una elección consciente y no un olvido.
//
// Se emite DESDE AQUÍ y no desde el validador de estructura de la spec 04 por una
// razón de tipo, no de gusto: el puerto `StepStructureRule` sólo sabe abortar
// —su veredicto es un `error`— y esto no debe abortar. Con eso, el obstáculo que
// la spec 04 §9.6 dejó anotado —«el punto de extensión no admite una
// advertencia»— deja de existir sin cambiar el puerto: la única regla que pedía
// severidades no lo necesita, porque nunca fue estructural.
//
// Su sitio natural es un `vex plan` o un linter de pipelinecode, y ahí se moverá
// cuando exista (spec 22).
func (h *StepRunnerHandler) warnIfOnlyExpires(request *StepRequestHandler, config StepConfig) {
	if !config.Rules().ExpiresWithoutInvalidating() {
		return
	}
	request.Emit(fmt.Sprintf(
		"advertencia: %s sólo declara reglas de expiración: revivirá su último resultado "+
			"durante toda su ventana de vigencia aunque su contenido cambie "+
			"(declara '%s' en 'rules' si no es lo que quieres)",
		request.StepNameExe(), RuleKindStateChanged))
}

// decide responde las dos preguntas de una vez: con qué huella se registrará
// este step y si hay que ejecutarlo. Devuelve también el motivo, que es lo que
// el usuario lee.
//
// Es el bucle de §5.6 completo, y lo que cambia con la spec 15 no es el bucle
// —lo dejó montado la 11— sino los dos `if` que había dentro: la comparación de
// huellas y la edad máxima eran del MOTOR, y ahora las declara el step.
//
// El fail-open de la spec 09 §5.3 —ante la duda, ejecutar— se conserva para la
// duda que se puede resolver ejecutando: no haber podido componer el material.
// Y se conserva que NO sea silencioso: confundir un caché roto con «el código
// cambió» era el defecto (c) de aquella spec.
//
// Lo que NO es fail-open es un registro ilegible. Ahí la duda no se puede
// resolver ejecutando sin arriesgar un recurso duplicado, así que el error sube
// y el step falla (spec 11 §5.6). Es la asimetría que la spec 02 §9.3 pidió
// conservar: en el índice, ilegible ⇒ ausente; en el registro, ilegible ⇒ error.
func (h *StepRunnerHandler) decide(
	ctx *context.Context,
	request *StepRequestHandler,
	commands []command.Command,
) (fingerprint cache.CacheKey, run bool, reason string, err error) {

	// La clave de POSICIÓN se compone antes que nada: sin ella no hay dónde leer
	// ni dónde escribir, y eso no es una duda que ejecutar resuelva. Es un
	// pipelinecode que no se puede almacenar, y se dice así.
	//
	// Es la clave del ámbito DECLARADO (spec 13 §5.4). Hasta la spec 13 era
	// siempre la del ambiente; para un step `scope: project` ahora es otra, que es
	// justo lo que hace que dos ambientes puedan compartirla.
	key, declarado, err := request.StateKey()
	if err != nil {
		return cache.CacheKey{}, false, "", fmt.Errorf(
			"componer la clave de estado de %s: %w", request.StepNameExe(), err)
	}

	// Sin `config.yaml` no hay ámbito, luego no hay clave, luego no hay dónde
	// constar: se ejecuta SIEMPRE y no se persiste nada (spec 13 §5.3). Se sale
	// con huella cero a propósito —no sólo evita el registro, evita también la
	// entrada de índice, que apunta a un registro que no va a existir.
	if !declarado {
		return cache.CacheKey{}, true, reasonNoScope, nil
	}

	// Sin reglas tampoco, y por otra razón (spec 15 §5.5): hay dónde recordarse,
	// pero no hay ninguna afirmación que guardar. El OR de un conjunto vacío es
	// falso y la lectura literal diría «revivir» — que es exactamente el defecto
	// que le da título a la spec 05, reaparecido por una vía nueva. La guarda vive
	// en `RuleSet`; esto es sólo salir antes de leer un registro que no se va a
	// usar.
	rules := request.StepConfig().Rules()
	if rules.IsEmpty() {
		return cache.CacheKey{}, true, reasonNoRules, nil
	}

	last, found, err := h.records.Last(ctx, key)
	if err != nil {
		return cache.CacheKey{}, false, "", fmt.Errorf(
			"leer el último registro de %s: %w", request.StepNameExe(), err)
	}

	// La huella se compone SÓLO si alguien la va a mirar. Un step que declara
	// nada más que `max_age` no compara contenidos —ésa es precisamente la
	// configuración de la que §5.4 avisa— así que calcularle una huella sería
	// trabajo cuyo resultado nadie lee, y anotarla dejaría en su registro una
	// afirmación que ninguna regla suya sostiene.
	//
	// Su registro sale entonces SIN huella, que es el mecanismo que ya existía
	// para «nunca revive» y no un camino nuevo.
	if watched, ok := rules.StateChanged(); ok {
		fingerprint, err = h.fingerprintOf(request, commands, watched)
		if err != nil {
			// Sin material no hay huella. El step se ejecuta y su registro se escribe
			// igual —lo que produjo es estado real— pero SIN huella, así que no
			// revivirá nunca. Es la asimetría de la spec 11 §4 aplicada a la duda:
			// guardar de más cuesta espacio, guardar de menos cuesta un recurso
			// duplicado.
			request.Emit(fmt.Sprintf(
				"advertencia: no se pudo componer la huella de %s: %v",
				request.StepNameExe(), err))
			return cache.CacheKey{}, true, reasonUndetermined, nil
		}
	}

	// Ausencia de registro ⇒ ejecutar, y esto va ANTES del OR a propósito: sin
	// registro no hay con qué comparar, así que `state_changed` diría «cambió» por
	// la razón equivocada y `max_age` no diría nada. Lo que falta no es una regla
	// que se cumpla: es el sujeto sobre el que se evalúan.
	if !found {
		return fingerprint, true, reasonNoRecord, nil
	}

	// EL OR. Cada regla es una razón independiente para desconfiar de lo
	// guardado, así que basta con que una se cumpla; exigir que coincidieran dos
	// razones independientes equivaldría a ignorar la primera que apareciera.
	//
	// Aquí es donde se paga la regresión declarada de la spec 11 §5.5: la
	// comparación de `state_changed` es contra el ÚLTIMO registro, no contra
	// «algún registro con esta huella», así que `A → B → A` re-ejecuta donde la
	// spec 10 acertaba. Es segura —re-ejecutar de más nunca omite un despliegue— y
	// el índice es exactamente la pieza que la haría barata de revertir el día que
	// se decida.
	kind, run := rules.RequiresRun(RuleSubject{
		Fingerprint: fingerprint.String(),
		Last:        last,
		Now:         request.StartedAt(),
	})
	if run {
		return fingerprint, true, reasonOf(kind), nil
	}

	// La procedencia es la razón de ser de `Provenance`: sin ella, «se revive»
	// dejaría sin respuesta «¿cuándo se probó esto por última vez?», que es la
	// afirmación de valor del motor (spec 10 §5.4). Que se lea aquí no la
	// convierte en parte de la decisión: la decisión ya está tomada.
	return fingerprint, false, fmt.Sprintf(" (ejecutado el %s por %s)",
		last.ProducedBy().At.UTC().Format(time.RFC3339),
		last.ProducedBy().ExecutionID), nil
}

// fingerprintOf compone la huella del step con el alcance que su regla
// `state_changed` declara.
//
// Material incompleto ⇒ error, nunca una huella degradada: una huella con un
// hueco es válida y COLISIONA con la de cualquier material al que le falte lo
// mismo, y esa colisión se manifiesta como un step que revive sin haberse
// ejecutado jamás.
func (h *StepRunnerHandler) fingerprintOf(
	request *StepRequestHandler,
	commands []command.Command,
	watched StateChangedRule) (cache.CacheKey, error) {

	material, err := NewCacheMaterial(request, commands, watched)
	if err != nil {
		return cache.CacheKey{}, err
	}
	return cache.NewCacheKey(material)
}
