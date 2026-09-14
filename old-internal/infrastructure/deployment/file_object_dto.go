// Package deployment es el almacén en disco de la INTENCIÓN: los objetos de
// despliegue y la cabeza del linaje de cada ambiente.
//
// Dos tiendas con dos reglas de vida y por eso dos archivos:
//
//	objects/   direccionado por contenido, write-once, PERMANENTE
//	lineage/   un puntero por ambiente, se SUSTITUYE
//
// El dominio (`internal/domain/deployment`) no escribe un byte; todo lo que hay
// aquí es la traducción a disco de lo que aquel modela.
package deployment

import (
	domDeployment "github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
)

// fileObjectSchemaVersion versiona la FORMA del archivo, no la regla que compone
// la identidad que lleva dentro. Son dos cosas distintas y conviene no fundirlas:
// `cnt-v1` dice cómo se compuso el `content_id` —cambiarlo cambia la dirección
// del objeto—, esto dice cómo está serializado.
const fileObjectSchemaVersion = 1

// FileObjectDTO es la forma en disco de un objeto de despliegue.
//
// # Lleva su `content_id` DENTRO, además de en la ruta
//
// La ruta es un hash, así que un archivo que no dice de qué contenido es no
// responde nada por sí solo. Y `record verify` (spec 22) tiene que poder
// recomputar la identidad y compararla con la que el archivo AFIRMA: si sólo
// estuviera en la ruta, la comprobación sería contra sí misma.
//
// # Y lleva la forma canónica completa
//
// Es lo que un tercero —el ingestor de la spec 26— tiene que poder reproducir
// bit a bit. Una regla que sólo se puede comprobar por su hash no se puede
// depurar cuando dos implementaciones discrepan, y aquí discrepar significa dos
// identidades permanentes distintas para el mismo despliegue.
//
// # No hay `ToDomain`, y su ausencia es una decisión
//
// Reconstruir un `Content` desde aquí sería LOSSY —las reglas del step se
// serializan por su forma canónica, no por su gramática— y un `Content`
// reconstruido a medias produce un `content_id` distinto del que el archivo
// declara: exactamente la clase de bug que un almacén direccionado por contenido
// no puede tener. Lo que un lector necesita —comparar el `content_id`
// recomputado con el declarado— se hace sobre `Canonical`, sin rehidratar nada.
// La lectura con caso de uso detrás es de la spec 22.
type FileObjectDTO struct {
	SchemaVersion int    `json:"schema_version"`
	ContentID     string `json:"content_id"`

	Subject     string `json:"subject"`
	Operation   string `json:"operation"`
	Destination string `json:"destination"`

	Source   FileSourceDTO        `json:"source"`
	Format   FileFormatDTO        `json:"format"`
	Steps    []FileStepContentDTO `json:"steps"`
	Complete bool                 `json:"complete"`

	// Canonical es la forma normativa sobre la que se calculó `content_id`.
	Canonical string `json:"canonical"`
}

// FileSourceDTO son las dos huellas de árbol y —separados de ellas— los dos
// commits, que son METADATO y no entran en la identidad.
type FileSourceDTO struct {
	Project  string `json:"project"`
	Pipeline string `json:"pipeline"`

	ProjectCommit  string `json:"project_commit,omitempty"`
	PipelineCommit string `json:"pipeline_commit,omitempty"`
}

type FileFormatDTO struct {
	SchemaVersion int  `json:"schema_version"`
	Declared      bool `json:"declared"`
}

// FileStepContentDTO es el material de un step dentro del objeto. Los parámetros
// llevan su DECLARACIÓN, nunca su valor resuelto (spec 14 §5.3): es la
// precondición dura de todo el registro, porque un valor de runtime no se puede
// saber por adelantado y la declaración de cómo se obtiene sí.
type FileStepContentDTO struct {
	StepID string `json:"step_id"`
	Scope  string `json:"scope"`
	Rules  string `json:"rules"`

	// Declaration es la huella `pipe-v1` de lo que el step declara hacer
	// (spec 27). El NOMBRE EN DISCO sigue siendo `instructions` y no se cambia:
	// `objects/` es write-once y permanente, así que los objetos emitidos antes
	// de la spec 27 —que llevan una `inst-v1` en esta misma clave— tienen que
	// seguir leyéndose. Renombrar la clave los dejaría con el campo vacío, y un
	// token ausente es CORRUPCIÓN para `record verify`, no un límite del lector
	// (22 §9.14). El token de dentro distingue las dos épocas, que es para lo que
	// existe.
	Declaration string `json:"instructions"`

	Parameters []FileParameterDTO `json:"parameters"`
}

type FileParameterDTO struct {
	Name        string `json:"name"`
	Declaration string `json:"declaration"`
}

func ToFileObjectDTO(
	content domDeployment.Content, metadata domDeployment.ObjectMetadata) FileObjectDTO {

	contentSteps := content.Steps()
	steps := make([]FileStepContentDTO, 0, len(contentSteps))
	for _, stepContent := range contentSteps {
		declarations := stepContent.Parameters()
		parameters := make([]FileParameterDTO, 0, len(declarations))
		for _, parameter := range declarations {
			parameters = append(parameters, FileParameterDTO{
				Name:        parameter.Name(),
				Declaration: parameter.Canonical(),
			})
		}
		steps = append(steps, FileStepContentDTO{
			StepID:      stepContent.StepID(),
			Scope:       stepContent.Config().Scope().String(),
			Rules:       stepContent.Config().Rules().Canonical(),
			Declaration: stepContent.Declaration().String(),
			Parameters:  parameters,
		})
	}

	return FileObjectDTO{
		SchemaVersion: fileObjectSchemaVersion,
		ContentID:     content.ID().String(),
		Subject:       content.Subject().String(),
		Operation:     content.Operation().String(),
		Destination:   content.Destination().String(),
		Source: FileSourceDTO{
			Project:        content.Source().Project().String(),
			Pipeline:       content.Source().Pipeline().String(),
			ProjectCommit:  metadata.ProjectCommit,
			PipelineCommit: metadata.PipelineCommit,
		},
		Format: FileFormatDTO{
			SchemaVersion: content.Format().SchemaVersion(),
			Declared:      content.Format().IsDeclared(),
		},
		Steps:     steps,
		Complete:  content.IsComplete(),
		Canonical: content.Canonical(),
	}
}
