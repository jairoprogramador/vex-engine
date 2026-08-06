package step

import (
	"context"
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

// Motivos por los que un paso se ejecuta. Son TRES, y sustituyen a la agregación
// de razones de cuatro reglas (spec 10 §5.3).
//
// La decisión entera se reduce a: existe entrada ⇒ saltar; no existe ⇒ ejecutar;
// expirada ⇒ ejecutar. No hay `Policy`, ni `PolicyBuilder`, ni `RuleRegistry`,
// ni `Decision`, ni `Evidence`, ni cuatro repositorios: cuando la decisión es
// «¿existe esta clave?», mantener la maquinaria de reglas sería estructura sin
// contenido.
const (
	reasonNoEntry      = "no consta que este contenido se haya ejecutado aquí"
	reasonExpired      = "la entrada de caché ha expirado"
	reasonUndetermined = "no se pudo determinar si ya se ejecutó"
)

type StepRunnerHandler struct {
	StepBaseHandler
	commandRepository PipelineCommandRepository
	entries           cache.Entries
}

var _ StepHandler = (*StepRunnerHandler)(nil)

func NewStepRunnerHandler(
	commandRepository PipelineCommandRepository,
	entries cache.Entries) StepHandler {

	return &StepRunnerHandler{
		StepBaseHandler:   StepBaseHandler{Next: nil},
		commandRepository: commandRepository,
		entries:           entries,
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
	// un paso con nombre desconocido (spec 05 §5.2). No se sustituyó por nada:
	// la clave se compone del MATERIAL, no del nombre, así que un `05-notify` ya
	// no necesita que el motor tenga sus comprobaciones cableadas. No hay entrada
	// para su clave, luego se ejecuta; al terminar bien, se escribe; la corrida
	// siguiente lo salta. Es el destino de P1, entregado por eliminación (spec 10
	// §5.3bis), y aquella medida de transición vivió sólo entre la 05 y la 10.
	key, run, reason := h.decide(ctx, request, commands)

	if !run {
		request.Emit(fmt.Sprintf("%s ya fue ejecutado y se mantiene sin cambios%s",
			request.StepNameExe(), reason))
		if h.Next != nil {
			return h.Next.Handle(ctx, request)
		}
		return nil
	}

	// La clave queda anotada ANTES de ejecutar, pero sólo se escribe si se llega
	// al final. Anotar no persiste nada: si el paso falla —o si el proceso muere
	// a mitad— la anotación muere con la cadena, y la corrida siguiente vuelve a
	// no encontrar entrada. Ésa es la ventana que la spec 09 cerró y que esta
	// spec sólo tiene que no reabrir al cambiar QUÉ se escribe.
	if !key.IsZero() {
		request.RecordCacheKey(key)
	}

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

// decide responde las dos preguntas de una vez: bajo qué clave va este paso y si
// hay que ejecutarlo. Devuelve también el motivo, que es lo que el usuario lee.
//
// Se conserva el fail-open de la spec 09 §5.3 —ante la duda, ejecutar— y se
// conserva que NO sea silencioso: un caché roto es algo que el usuario tiene que
// ver, y confundirlo con «el código cambió» era el defecto (c) de aquella spec.
// Lo que ya no hace falta es un tercer estado en un enum: aquí la duda tiene un
// solo origen —no se pudo consultar— y viaja en el motivo.
func (h *StepRunnerHandler) decide(
	ctx *context.Context,
	request *StepRequestHandler,
	commands []command.Command,
) (key cache.CacheKey, run bool, reason string) {

	material, err := NewCacheMaterial(request, commands)
	if err == nil {
		key, err = cache.NewCacheKey(material)
	}
	if err != nil {
		// Sin material no hay clave, y sin clave no se escribe entrada. Que la
		// ausencia de entrada implique ejecutar es lo que conserva —por
		// construcción, sin código que la defienda— la semántica «sin evidencia
		// ⇒ ejecutar» de la spec 05 §5.1: una clave que nadie escribió no puede
		// afirmar que nada cambió.
		request.Emit(fmt.Sprintf(
			"advertencia: no se pudo componer la clave de caché de %s: %v",
			request.StepNameExe(), err))
		return cache.CacheKey{}, true, reasonUndetermined
	}

	entry, found, err := h.entries.Get(ctx, key)
	if err != nil {
		// ADVERTENCIA VISIBLE, no razón de negocio. Un fallo de infraestructura
		// no es «el contenido cambió». La clave SÍ se devuelve: el paso se
		// ejecuta y, si termina bien, deja su entrada — no haber podido leer no
		// es motivo para no escribir.
		request.Emit(fmt.Sprintf(
			"advertencia: no se pudo consultar el caché de %s: %v",
			request.StepNameExe(), err))
		return key, true, reasonUndetermined
	}

	if !found {
		return key, true, reasonNoEntry
	}

	// El TTL es metadato de la entrada, no material de la clave: la entrada
	// nueva se escribirá bajo la MISMA clave, sustituyendo a la caducada en su
	// sitio.
	if entry.IsExpired(request.StartedAt()) {
		return key, true, reasonExpired
	}

	// La procedencia es la razón de ser de `Provenance`: sin ella, «se salta»
	// con una clave opaca dejaría sin respuesta «¿cuándo se probó esto por
	// última vez?», que es la afirmación de valor del motor (spec 10 §5.4).
	return key, false, fmt.Sprintf(" (ejecutado el %s por %s)",
		entry.ProducedBy.At.UTC().Format(time.RFC3339),
		entry.ProducedBy.ExecutionID)
}
