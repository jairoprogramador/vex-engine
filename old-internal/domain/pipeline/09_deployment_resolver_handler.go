package pipeline

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
)

// DeploymentResolverHandler es el propósito explícito de la cadena de pipeline:
// **antes de ejecutar el primer comando, el motor sabe qué va a hacer y en qué
// posición de la historia de este ambiente cae** (spec 18 §3).
//
// Hasta aquí la cadena preparaba el terreno y ejecutaba; ahora primero DECLARA
// qué va a hacer y luego lo hace. Es la diferencia entre un script y un modelo.
//
// # Lo que hace, en orden (§5.1)
//
//  1. carga `commands.yaml`, `config.yaml` y `variables/<ambiente>/<paso>.yaml`
//     de TODOS los steps de la operación — no sólo del pedido;
//  2. calcula la huella del árbol del pipelinecode con la MISMA regla que la del
//     proyecto;
//  3. compone `Content`, deriva `content_id`, resuelve el `parent` del linaje y
//     deriva `deployment_id`;
//  4. escribe el objeto (write-once), avanza el linaje y abre la tira de hechos
//     con `attempt_started`.
//
// # Por qué está en la posición 09 y no en la 03
//
// Porque la identidad se calcula sobre la fuente REALMENTE USADA (§5.4). El
// clonador puede haber reutilizado un clon dentro de su ventana o haber caído en
// uno viejo porque el remoto no respondió, y un `content_id` compuesto antes de
// esa decisión afirmaría que se ejecutó una versión del pipeline que no fue la
// que corrió.
//
// # Y el fallo rápido se cobra aquí como beneficio lateral
//
// Un `04-deploy/commands.yaml` malformado hacía fallar la ejecución DESPUÉS de
// haber corrido los tests, provisionado infraestructura y construido un
// artefacto. A partir de aquí, **una comprobación sobre el pipelinecode que se
// haga por step dentro de la cadena 2 es un error de diseño**: el input ya está
// disponible antes de ejecutar nada.
//
// Con un borde declarado: se cargan los steps de `request.Steps()`, que es
// `steps[:pedido+1]`. Un `05-notify` roto NO hace fallar un `supply`, y es
// deliberado — no forma parte de la operación, y meterlo en el objeto haría que
// `deploy` y `supply` tuvieran identidades contaminadas la una por la otra—. Lo
// que sí ve el repositorio ENTERO es el validador de estructura de la spec 04.
type DeploymentResolverHandler struct {
	PipelineBaseHandler

	commands     domStep.PipelineCommandRepository
	configs      domStep.StepConfigRepository
	declarations domStep.VarsPipelineRepository
	trees        domStep.StepTreeRepository
	manifests    ManifestRepository
	fingerprints ContentFingerprint

	// loaded es el destino de la carga: lo que la cadena de step consumirá en vez
	// de volver a leer del disco (§5.2). Una sola lectura, una sola verdad.
	loaded *domStep.LoadedPipelinecode

	objects  deployment.ObjectStore
	lineages deployment.LineageStore
	emitter  *record.Emitter

	// El rollback de la spec 28 entra AQUÍ y no en la cadena de step, y las tres
	// piezas que necesita ya estaban en este handler: la lista de steps de la
	// operación —para cotejarla con la del destino—, el `content_id` que se acaba
	// de componer —para decir si sigue siendo la misma intención— y el hecho de
	// apertura, que es donde el ancla se confirma.
	//
	// `rollbacks` resuelve el ancla; `current` es donde se instala la política que
	// la cadena de step usará. Las dos son del destino configurado, y la segunda
	// es la misma indirección que `loaded`: se cablea antes de leer el
	// `RequestInput`, así que la elección de política ocurre aquí.
	rollbacks RollbackResolver
	current   *domStep.CurrentRecords
}

var _ PipelineHandler = (*DeploymentResolverHandler)(nil)

func NewDeploymentResolverHandler(
	commands domStep.PipelineCommandRepository,
	configs domStep.StepConfigRepository,
	declarations domStep.VarsPipelineRepository,
	trees domStep.StepTreeRepository,
	manifests ManifestRepository,
	fingerprints ContentFingerprint,
	loaded *domStep.LoadedPipelinecode,
	objects deployment.ObjectStore,
	lineages deployment.LineageStore,
	emitter *record.Emitter,
	rollbacks RollbackResolver,
	current *domStep.CurrentRecords) PipelineHandler {

	return &DeploymentResolverHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		commands:            commands,
		configs:             configs,
		declarations:        declarations,
		trees:               trees,
		manifests:           manifests,
		fingerprints:        fingerprints,
		loaded:              loaded,
		objects:             objects,
		lineages:            lineages,
		emitter:             emitter,
		rollbacks:           rollbacks,
		current:             current,
	}
}

func (h *DeploymentResolverHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	request.Emit("resolviendo la identidad del despliegue")

	content, err := h.compose(ctx, request)
	if err != nil {
		return err
	}

	// EL ANCLA SE RESUELVE AQUÍ, y va antes de escribir el objeto y de avanzar el
	// linaje a propósito (spec 28 §5.5): un destino inválido se rechaza **antes
	// del primer step**, y rechazar después de avanzar la cabeza dejaría una
	// posición ocupada por una ejecución que nunca corrió.
	//
	// Necesita el contenido ya compuesto porque lo que valida es la lista de steps
	// de la operación de HOY contra la que declaraba el destino, así que no puede
	// ir antes; y necesita ir antes del objeto, así que su sitio es exactamente
	// éste.
	anchor, err := h.resolveRollback(ctx, request, content)
	if err != nil {
		return err
	}

	// El objeto se escribe ANTES de situarlo en la historia y antes de abrir la
	// tira, y el orden importa en las dos direcciones. Una cabeza de linaje o un
	// `attempt_started` que apuntan a un objeto que no está en disco dejan el
	// registro afirmando una intención que no se puede leer. Al revés —objeto sin
	// posición, o con posición y sin hechos— es sólo un intento que murió muy
	// pronto, que es un estado normal y legible.
	if err := h.objects.Put(ctx, content, deployment.ObjectMetadata{
		ProjectCommit:  request.ProjectHeadHash(),
		PipelineCommit: request.PipelineHeadHash(),
	}); err != nil {
		return fmt.Errorf("registrar la intención del despliegue: %w", err)
	}

	deploymentID, attempt, err := h.place(ctx, content)
	if err != nil {
		return err
	}

	if err := h.emitter.Open(ctx,
		record.EventStream{
			Deployment:  deploymentID,
			ExecutionID: request.ExecutionID(),
		},
		attempt,
	); err != nil {
		return fmt.Errorf("abrir el registro del intento: %w", err)
	}

	// `actor` y `runner` son CIRCUNSTANCIA, y por eso viajan en el hecho y no en
	// el objeto: quién lanzó el despliegue y sobre qué máquina corrió no cambian
	// qué se pretendía hacer, y meterlos en la identidad haría que dos
	// ejecuciones idénticas de dos personas distintas fueran objetos distintos.
	//
	// `runner` es lo único de los dos que el motor sabe hoy —la imagen del
	// runtime que el `RequestInput` declara—. **`actor` se queda vacío a
	// propósito**: el contrato de entrada no lo trae, y rellenarlo con el usuario
	// del proceso diría «vex» en todo despliegue remoto, que es peor que no decir
	// nada. Lo llena quien añada el campo al `RequestInput`, que es un cambio de
	// contrato y no de este repo (specs 23 y 26).
	//
	// `rollback_to` sí es del contrato de entrada y viaja LLENO cuando lo hubo
	// (spec 28 §5.5): es la confirmación de que el motor entendió el ancla, y va
	// en el hecho —que se empuja— y no en el objeto, porque meterlo en el objeto
	// cambiaría el `content_id` y que R y E compartan `content_id` es la mitad del
	// diseño.
	if err := h.emitter.Emit(ctx, record.AttemptStarted{
		Deployment: deploymentID,
		Runner:     request.Runner(),
		RollbackTo: anchor.Target(),
	}); err != nil {
		return fmt.Errorf("registrar el inicio del intento: %w", err)
	}

	request.SetDeployment(content, deploymentID, attempt)
	request.Emit(fmt.Sprintf("  - Contenido: %s", content.ID()))
	request.Emit(fmt.Sprintf("  - Despliegue: %s", deploymentID))
	if !content.IsComplete() {
		// Se emite igual, MARCADO (§5.5). «Este material está incompleto» es un
		// hecho; omitir el objeto sería perder la historia, y emitirlo como
		// completo sería mentir.
		request.Emit(fmt.Sprintf(
			"  - Material incompleto: el pipelinecode no trae '%s', así que no declara"+
				" de dónde salen sus variables", manifestFileNameForMessage))
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}

// resolveRollback resuelve, valida e INSTALA el ancla de un rollback (spec 28).
//
// Devuelve el ancla cero cuando no se pidió ninguno, que es el caso normal: sin
// `rollback_to` la ejecución se comporta exactamente como antes de la spec.
//
// # Lo que aborta y lo que sólo avisa, que es la decisión que la spec dejó abierta
//
//	ABORTA   el destino no existe, no fue exitoso, está incompleto, no se puede
//	         enlazar su objeto, o declara OTRA lista de steps —o los mismos con
//	         otro ámbito— que la operación de hoy. Todos comparten forma: el ancla
//	         no apuntaría a nada, y un rollback que no ancla y no lo dice es el
//	         modo de fallo que la spec existe para evitar.
//	AVISA    el `content_id` de hoy no coincide con el del destino. Ocurre cuando
//	         el pipelinecode cambió en el remoto y el clon se refrescó (spec 18,
//	         recuadro): R deja de ser estrictamente la misma intención que E. NO
//	         aborta porque §7 exige lo contrario — un step cuyo `commands.yaml`
//	         cambió entre E y R **se ejecuta**, aunque su registro anclado exista.
//	         Un rollback no relaja ninguna comprobación, y tampoco es un permiso
//	         para negarse a ejecutar lo que cambió.
//
// # La política se instala DESPUÉS de validar, y una sola vez
//
// A partir de aquí las tres lecturas del registro de la cadena de step —la carga
// del mapa acumulado, el resolutor de `resolve: state` y el bucle de decisión—
// leen el registro ANCLADO en vez del último. El ancla es inmutable, así que lo
// que R escribe no cambia de qué registro parten los steps que le quedan (§7).
func (h *DeploymentResolverHandler) resolveRollback(
	ctx *context.Context,
	request *PipelineRequestHandler,
	content deployment.Content) (deployment.RollbackAnchor, error) {

	pedido := request.Rollback()
	if pedido.IsZero() {
		return deployment.RollbackAnchor{}, nil
	}

	// Se vuelve a componer con el MISMO constructor que el borde ya usó, así que
	// aquí no puede fallar: lo que se gana es el tipo, no una segunda definición
	// de «bien formado».
	target, err := deployment.ParseRollbackTarget(pedido.DeploymentID, pedido.Attempt)
	if err != nil {
		return deployment.RollbackAnchor{}, fmt.Errorf("componer el destino del rollback: %w", err)
	}

	request.Emit(fmt.Sprintf("volviendo a %s", target))

	anchor, err := h.rollbacks.Resolve(ctx, target)
	if err != nil {
		return deployment.RollbackAnchor{}, fmt.Errorf("resolver el rollback: %w", err)
	}
	if err := anchor.Validate(content); err != nil {
		return deployment.RollbackAnchor{}, fmt.Errorf("resolver el rollback: %w", err)
	}

	if !anchor.Matches(content) {
		request.Emit(fmt.Sprintf(
			"  - advertencia: el contenido de hoy (%s) no es el de %s (%s): el pipelinecode o el"+
				" proyecto cambiaron, así que esto se parece a un rollback y no lo es del todo."+
				" El estado sí se ancla; lo que cambió se ejecuta",
			content.ID(), target, anchor.ContentID()))
	}

	anclados := 0
	for _, step := range anchor.Steps() {
		if step.Remembers() {
			anclados++
		}
	}
	request.Emit(fmt.Sprintf(
		"  - Ancla: %d de %d steps vuelven a su registro de entonces",
		anclados, len(anchor.Steps())))

	h.current.UseAnchor(anchor)
	return anchor, nil
}

// compose lee el material de la operación y arma la intención congelada.
func (h *DeploymentResolverHandler) compose(
	ctx *context.Context, request *PipelineRequestHandler) (deployment.Content, error) {

	steps, err := h.loadSteps(ctx, request)
	if err != nil {
		return deployment.Content{}, err
	}

	source, err := h.sourceOf(request)
	if err != nil {
		return deployment.Content{}, err
	}

	format, err := h.formatOf(ctx, request)
	if err != nil {
		return deployment.Content{}, err
	}

	subject, err := deployment.NewSubject(request.ProjectUrl())
	if err != nil {
		return deployment.Content{}, err
	}
	operation, err := deployment.NewOperation(request.StepName())
	if err != nil {
		return deployment.Content{}, err
	}
	destination, err := deployment.NewDestination(request.Environment())
	if err != nil {
		return deployment.Content{}, err
	}

	content, err := deployment.NewContent(subject, operation, destination, source, format, steps)
	if err != nil {
		return deployment.Content{}, fmt.Errorf("componer la intención del despliegue: %w", err)
	}
	return content, nil
}

// loadSteps lee los TRES archivos de cada step de la operación y deja el
// material donde la cadena de step lo va a consumir.
//
// El fallo rápido se cobra aquí: un `commands.yaml` malformado de un step
// posterior aborta la ejecución antes del primer comando del primero.
func (h *DeploymentResolverHandler) loadSteps(
	ctx *context.Context, request *PipelineRequestHandler) ([]deployment.StepContent, error) {

	pipelineLocalPath := request.PipelineLocalPath()
	environment := request.Environment()

	steps := make([]deployment.StepContent, 0, len(request.Steps()))
	for _, stepName := range request.Steps() {
		stepID := stepName.FullName()

		commands, err := h.commands.Get(ctx, pipelineLocalPath, stepID)
		if err != nil {
			return nil, fmt.Errorf("cargar los comandos de '%s': %w", stepID, err)
		}
		config, err := h.configs.Get(ctx, pipelineLocalPath, stepID)
		if err != nil {
			return nil, fmt.Errorf("cargar la configuración de '%s': %w", stepID, err)
		}
		declarations, err := h.declarations.Get(ctx, pipelineLocalPath, environment, stepName.Name())
		if err != nil {
			return nil, fmt.Errorf("cargar las variables de '%s': %w", stepID, err)
		}

		loaded := domStep.LoadedStep{
			Commands:     commands,
			Config:       config,
			Declarations: declarations,
		}

		// La huella de la declaración se compone AQUÍ, con los tres archivos
		// recién leídos y el directorio del step a mano, y se comparte con la
		// cadena de step: es el único punto de traducción de `commands.yaml`,
		// `config.yaml` y `variables/` a material de huella (spec 27 §5.2').
		//
		// Que se pueda componer antes de ejecutar es lo que el cambio de material
		// de la spec 27 entrega: mientras la huella dependió del mapa acumulado
		// RESUELTO, no existía hasta que el step estaba abierto.
		tree, err := h.trees.Get(ctx, pipelineLocalPath, stepID)
		if err != nil {
			return nil, fmt.Errorf("abrir el directorio de '%s': %w", stepID, err)
		}
		declaration, err := domStep.NewDeclarationFingerprint(loaded, tree)
		if err != nil {
			return nil, fmt.Errorf("huella de la declaración de '%s': %w", stepID, err)
		}
		loaded.Declaration = declaration

		h.loaded.Put(stepID, loaded)

		stepContent, err := deployment.NewStepContent(stepID, config, declaration, declarations)
		if err != nil {
			return nil, fmt.Errorf("material de '%s': %w", stepID, err)
		}
		steps = append(steps, stepContent)
	}
	return steps, nil
}

// sourceOf son las DOS huellas de árbol, calculadas con la misma regla.
//
// La del proyecto ya está calculada —la puso el handler 08 en su forma canónica
// con prefijo— y se vuelve a parsear en vez de recalcularla: una segunda pasada
// sobre el árbol podría ver otro contenido, y la huella que decide el salto de un
// step tiene que ser la misma que identifica el despliegue.
//
// La del pipelinecode no cuesta código nuevo (§5.1, paso 3): otro
// `DirTreeSource` sobre la ruta del clon y la misma `Compute`. Que sea LA MISMA
// regla es lo único que impide que alguien introduzca una variante «para el
// pipeline», y con ella la comparabilidad entre organizaciones.
func (h *DeploymentResolverHandler) sourceOf(request *PipelineRequestHandler) (deployment.Source, error) {
	project, err := fingerprint.Parse(request.ProjectStatus())
	if err != nil {
		return deployment.Source{}, fmt.Errorf("huella del árbol del proyecto: %w", err)
	}

	pipeline, err := h.fingerprints.FromDirectory(request.PipelineLocalPath())
	if err != nil {
		return deployment.Source{}, fmt.Errorf("huella del árbol del pipelinecode: %w", err)
	}

	return deployment.NewSource(project, pipeline)
}

// formatOf traduce el manifiesto a la marca de formato del objeto.
//
// Es una línea, y esa línea es la razón de que `deployment.Format` exista en vez
// de reutilizarse `pipeline.Manifest`: el resolutor vive en este paquete, así que
// `pipeline` importa `deployment` y el import de vuelta sería un ciclo
// (spec 17 §9.2).
func (h *DeploymentResolverHandler) formatOf(
	ctx *context.Context, request *PipelineRequestHandler) (deployment.Format, error) {

	manifest, err := h.manifests.Get(ctx, request.PipelineLocalPath())
	if err != nil {
		return deployment.Format{}, fmt.Errorf("leer el formato del pipelinecode: %w", err)
	}
	return deployment.NewFormat(manifest.SchemaVersion(), manifest.IsDeclared())
}

// place resuelve dónde cae este contenido en la historia de su ambiente.
//
// El linaje se lee, se avanza y se guarda: **el mismo contenido dos veces produce
// dos `deployment_id` distintos**, porque el segundo cuelga del primero. No es un
// defecto —es la diferencia entre las dos identidades— y de ahí sale la regla que
// gobierna el resto del motor: la decisión de saltar un step NO puede consultar
// `deployment_id`.
//
// El intento es SIEMPRE el primero, y eso merece una frase porque parece un
// pendiente y no lo es: como el linaje avanza en cada ejecución, este motor no
// deriva dos veces el mismo `deployment_id`, así que no hay ningún segundo
// intento que numerar. Derivarlo de un conteo —lectura seguida de escritura, sin
// atomicidad— daría el mismo número a dos ejecuciones concurrentes (spec 17
// §5.8), y por eso `attempt` tampoco nombra el archivo de hechos.
func (h *DeploymentResolverHandler) place(
	ctx *context.Context, content deployment.Content) (deployment.DeploymentID, deployment.Attempt, error) {

	lineage, err := h.lineages.Head(ctx, content.Subject(), content.Destination())
	if err != nil {
		return deployment.DeploymentID{}, deployment.Attempt{}, fmt.Errorf(
			"leer la historia de '%s': %w", content.Destination(), err)
	}

	advanced, deploymentID, err := lineage.Advance(content.ID())
	if err != nil {
		return deployment.DeploymentID{}, deployment.Attempt{}, fmt.Errorf(
			"situar el despliegue en la historia de '%s': %w", content.Destination(), err)
	}

	if err := h.lineages.Save(ctx, advanced); err != nil {
		return deployment.DeploymentID{}, deployment.Attempt{}, fmt.Errorf(
			"avanzar la historia de '%s': %w", content.Destination(), err)
	}

	return deploymentID, deployment.FirstAttempt(), nil
}
