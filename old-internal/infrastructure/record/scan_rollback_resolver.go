package record

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	domDeployment "github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	domPipeline "github.com/jairoprogramador/vex-engine/old-internal/domain/pipeline"
	domRecord "github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	deploymentInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/deployment"
)

var _ domPipeline.RollbackResolver = (*ScanRollbackResolver)(nil)

// ScanRollbackResolver compone el ancla de un rollback leyendo los hechos y el
// objeto de la ejecución destino (spec 28 §5.3).
//
// # Las dos mitades del ancla vienen de sitios distintos, y hacen falta las dos
//
//	de los HECHOS   `evidence_from` de cada `step_finished`: QUÉ registro estuvo
//	                vigente. Lo emite la spec 19 en las dos mitades —cuando el
//	                step revivió y cuando ejecutó—, y sin la segunda esto no
//	                sería implementable sin derivar por fechas.
//	del OBJETO      la lista de steps de aquella operación y el ámbito declarado
//	                de cada uno. Es lo que se coteja con la operación de hoy, y
//	                sin ello un rollback a un reparto de steps que ya no existe
//	                no anclaría ninguno y no lo diría (spec 24, recuadro).
//
// # Y las dos se leen del DESTINO
//
// No del área de trabajo, que es lo que `vexd record` lee por defecto. Ahí la
// diferencia es de propósito —«¿qué ocurrió aquí?» frente a «¿qué se publicó?»—
// y aquí es de corrección: el registro anclado se resuelve contra `state/`, que
// vive en el destino, así que leer los hechos de otra raíz sería resolver un
// `record_id` contra un almacén que no es el que lo escribió (spec 21,
// recuadro). Con `type: local` las dos mitades caen en el mismo volumen y
// coinciden; que sea el destino y no el área de trabajo es además lo que hace
// que un rollback pueda anclarse en una ejecución de OTRA máquina.
//
// La consecuencia declarada: una ejecución cuyo empuje nunca llegó al destino no
// es anclable, y el diagnóstico lo dice nombrando la raíz. Es la respuesta
// correcta —un rollback que no encuentra el registro de ayer no puede degradar a
// «despliega lo de hoy»— y no un límite que haya que rodear.
type ScanRollbackResolver struct {
	projection  *ScanProjection
	objectsRoot string
}

// NewScanRollbackResolver construye el resolutor sobre una raíz —la del destino.
func NewScanRollbackResolver(base string) *ScanRollbackResolver {
	eventsRoot := filepath.Join(base, EventsDirName)
	objectsRoot := filepath.Join(base, deploymentInfra.ObjectsDirName)
	return &ScanRollbackResolver{
		projection:  NewScanProjection(eventsRoot, objectsRoot),
		objectsRoot: objectsRoot,
	}
}

// Resolve compone el ancla, o rechaza el destino nombrando el motivo.
func (r *ScanRollbackResolver) Resolve(
	ctx *context.Context,
	target domDeployment.RollbackTarget) (domDeployment.RollbackAnchor, error) {

	if target.IsZero() {
		// No pedir rollback no es un caso que haya que diagnosticar.
		return domDeployment.RollbackAnchor{}, nil
	}

	result, err := r.projection.Attempt(ctx, target.Deployment(), target.Attempt())
	if err != nil {
		if errors.Is(err, domRecord.ErrAttemptNoConsta) {
			// El centinela importa más de lo que parece: `Fold(nil)` pliega a
			// `interrupted`, así que sin él un `deployment_id` mal escrito se leería
			// como «esa ejecución se interrumpió» en vez de «no existe», y las dos se
			// rechazan por motivos distintos.
			return domDeployment.RollbackAnchor{}, fmt.Errorf(
				"no consta ningún hecho del intento %s en %s: nada a lo que volver",
				target, r.projection.eventsRoot)
		}
		return domDeployment.RollbackAnchor{}, fmt.Errorf("leer los hechos de %s: %w", target, err)
	}

	// §5.2 hecho código, y ya existía: terminó bien, todos los steps cerraron y
	// ninguno falló. Un step revivido o saltado cuenta como correcto — excluirlo
	// haría que una ejecución fuera peor destino cuanto mejor funcionó el motor.
	if !result.IsValidTarget() {
		return domDeployment.RollbackAnchor{}, fmt.Errorf(
			"%s no es un destino válido de rollback: %s", target, porQueNoSirve(result))
	}

	object, err := r.objetoDe(target.Deployment())
	if err != nil {
		return domDeployment.RollbackAnchor{}, err
	}

	contentID, err := domDeployment.ParseContentID(object.Object.ContentID)
	if err != nil {
		return domDeployment.RollbackAnchor{}, fmt.Errorf(
			"el objeto de %s no declara una identidad de contenido legible: %w", target, err)
	}

	steps := make([]domDeployment.AnchoredStep, 0, len(object.Object.Steps))
	for _, declarado := range object.Object.Steps {
		alcanzado, ok := result.Step(declarado.StepID)
		if !ok {
			// La completitud que `IsValidTarget` no puede comprobar sola (spec 22,
			// recuadro): un step que nunca se abrió NO aparece en el pliegue, así que
			// el cotejo exhaustivo es contra la lista que declara el OBJETO. Hoy no
			// debería ocurrir —un desenlace `succeeded` implica que la operación
			// entera corrió— y por eso es un rechazo y no un ancla a medias.
			return domDeployment.RollbackAnchor{}, fmt.Errorf(
				"%s declara el step '%s' y no consta que se alcanzara: el intento está incompleto",
				target, declarado.StepID)
		}

		anchored := domDeployment.AnchoredStep{
			StepID: declarado.StepID,
			Scope:  declarado.Scope,
		}
		// La evidencia CERO es legítima y frecuente: los tres casos que ejecutan y
		// no persisten —sin `config.yaml`, sin comandos, sin `rules`— no invocan
		// ningún registro. Para ésos el rollback no ancla nada, y no hace falta:
		// son steps que se ejecutan siempre (§5.2).
		if !alcanzado.Evidence.IsZero() {
			anchored.Key = alcanzado.Evidence.StateKey
			anchored.RecordID = alcanzado.Evidence.RecordID
		}
		steps = append(steps, anchored)
	}

	anchor, err := domDeployment.NewRollbackAnchor(target, contentID, steps)
	if err != nil {
		return domDeployment.RollbackAnchor{}, err
	}
	return anchor, nil
}

// objetoDe enlaza el despliegue con el objeto del que salió.
//
// Los candidatos a padre son TODOS los despliegues de la raíz y no el pedido, y
// es la diferencia entre funcionar y no funcionar: `deployment_id =
// H(content_id, parent)` y todo despliegue menos el primero de su ambiente
// cuelga del anterior (spec 22 §9.1).
func (r *ScanRollbackResolver) objetoDe(
	id domDeployment.DeploymentID) (deploymentInfra.IndexedObject, error) {

	index, err := deploymentInfra.ScanObjects(r.objectsRoot)
	if err != nil {
		return deploymentInfra.IndexedObject{}, fmt.Errorf(
			"recorrer los objetos de %s: %w", r.objectsRoot, err)
	}
	conocidos, err := r.projection.Deployments()
	if err != nil {
		return deploymentInfra.IndexedObject{}, err
	}
	conocidos = append(conocidos, id)

	object, enlazado := index.Link(conocidos)[id.String()]
	if !enlazado {
		return deploymentInfra.IndexedObject{}, fmt.Errorf(
			"no se puede enlazar el objeto de %s desde %s: falta el padre del que deriva su"+
				" posición, o el objeto no está en esta raíz. Sin él no hay lista de steps que"+
				" cotejar, y seguir con un content_id que no es el suyo dejaría de ser un rollback",
			id, r.objectsRoot)
	}
	return object, nil
}

// porQueNoSirve traduce el pliegue al motivo por el que ese intento no es un
// destino válido.
//
// Se enumera en vez de decir «no es válido» porque el usuario eligió ese intento
// de un listado que los MARCA (`record history`), así que llegar aquí significa
// que algo no cuadra y hay que decir qué.
func porQueNoSirve(result domRecord.AttemptResult) string {
	if result.Status != domRecord.AttemptSucceeded {
		return fmt.Sprintf("terminó '%s' y sólo se puede volver a un intento 'succeeded'"+
			" (último step alcanzado: %s)", result.Status, vacio(result.LastStep))
	}
	if len(result.Steps) == 0 {
		return "no consta que llegara a abrirse ni un step"
	}
	for _, step := range result.Steps {
		if !step.Finished {
			return fmt.Sprintf("el step '%s' quedó abierto: nadie escribió su cierre", step.StepID)
		}
		if step.Status == command.StepFailure {
			return fmt.Sprintf("el step '%s' falló", step.StepID)
		}
	}
	return "no cumple el criterio de destino válido"
}

func vacio(valor string) string {
	if valor == "" {
		return "(no consta)"
	}
	return valor
}
