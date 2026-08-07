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
// La decisión entera se reduce a: **leer el último registro de la clave de
// posición y comparar su huella**. No hay `Policy`, ni `PolicyBuilder`, ni
// `RuleRegistry`, ni `Decision`, ni `Evidence`, ni cuatro repositorios.
//
// Y no consulta el índice. Ésa es la propiedad que la spec 11 §5.5 compra y que
// el harness comprueba borrándolo: `rm -rf $HOME/.vex/cache` no cambia una sola
// de estas decisiones.
const (
	reasonNoRecord     = "no consta que este step se haya ejecutado aquí"
	reasonChanged      = "el contenido del step cambió desde la última ejecución"
	reasonExpired      = "el último registro del step ha caducado"
	reasonUndetermined = "no se pudo determinar si ya se ejecutó"
)

// defaultMaxAge es cuánto vale el último registro antes de exigir una revisión.
//
// Son los 30 días que la regla de tiempo aplicaba SÓLO al step `test` y que la
// spec 10 extendió a todos como `cache.DefaultTTL`. Cambia de sitio con la
// spec 11 porque cambia de sujeto: la expiración es una propiedad del REGISTRO
// —de su edad— y no del índice, que ya no decide nada. La spec 15 la retira de
// aquí y la declara por step (`max_age`), y entonces un step sin `max_age`
// dejará de caducar.
//
// La edad se mide contra el último registro de la clave, que con dos pipelines
// sobre el mismo proyecto puede ser del otro: residuo declarado en la spec 11
// §5.2, y el mismo caso que la advertencia del linter de la spec 15 §5.5.
const defaultMaxAge = 30 * 24 * time.Hour

type StepRunnerHandler struct {
	StepBaseHandler
	commandRepository PipelineCommandRepository
	records           state.Records
}

var _ StepHandler = (*StepRunnerHandler)(nil)

func NewStepRunnerHandler(
	commandRepository PipelineCommandRepository,
	records state.Records) StepHandler {

	return &StepRunnerHandler{
		StepBaseHandler:   StepBaseHandler{Next: nil},
		commandRepository: commandRepository,
		records:           records,
	}
}

func (h *StepRunnerHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	commands, err := h.commandRepository.Get(ctx, request.PipelineLocalPath(), request.StepFullName())
	if err != nil {
		return fmt.Errorf("cargar commands: %w", err)
	}

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

// decide responde las dos preguntas de una vez: con qué huella se registrará
// este step y si hay que ejecutarlo. Devuelve también el motivo, que es lo que
// el usuario lee.
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
	key, err := request.EnvironmentStateKey()
	if err != nil {
		return cache.CacheKey{}, false, "", fmt.Errorf(
			"componer la clave de estado de %s: %w", request.StepNameExe(), err)
	}

	last, found, err := h.records.Last(ctx, key)
	if err != nil {
		return cache.CacheKey{}, false, "", fmt.Errorf(
			"leer el último registro de %s: %w", request.StepNameExe(), err)
	}

	material, err := NewCacheMaterial(request, commands)
	if err == nil {
		fingerprint, err = cache.NewCacheKey(material)
	}
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

	if !found {
		return fingerprint, true, reasonNoRecord, nil
	}

	if !last.Revives(fingerprint.String()) {
		// Aquí es donde se paga la regresión declarada de la spec 11 §5.5: la
		// comparación es contra el ÚLTIMO registro, no contra «algún registro con
		// esta huella», así que `A → B → A` re-ejecuta donde la spec 10 acertaba.
		// Es segura —re-ejecutar de más nunca omite un despliegue— y el índice es
		// exactamente la pieza que la haría barata de revertir el día que se
		// decida: la pregunta «¿existe algún registro con esta huella?» ya está
		// respondida ahí.
		return fingerprint, true, reasonChanged, nil
	}

	// El borde es exclusivo: el registro caduca CUANDO se alcanza su instante,
	// no después. Es la misma frontera que comparaba la regla de tiempo.
	if !request.StartedAt().Before(last.ProducedBy().At.Add(defaultMaxAge)) {
		return fingerprint, true, reasonExpired, nil
	}

	// La procedencia es la razón de ser de `Provenance`: sin ella, «se revive»
	// dejaría sin respuesta «¿cuándo se probó esto por última vez?», que es la
	// afirmación de valor del motor (spec 10 §5.4). Que se lea aquí no la
	// convierte en parte de la decisión: la decisión ya está tomada.
	return fingerprint, false, fmt.Sprintf(" (ejecutado el %s por %s)",
		last.ProducedBy().At.UTC().Format(time.RFC3339),
		last.ProducedBy().ExecutionID), nil
}
