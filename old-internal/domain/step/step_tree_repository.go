package step

import (
	"context"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
)

// StepTreeRepository abre el árbol del directorio de un step del pipelinecode.
//
// Es el cuarto puerto de lectura del pipelinecode, junto a los de `commands.yaml`,
// `config.yaml` y `variables/<ambiente>/<paso>.yaml`, y existe porque desde la
// spec 27 el directorio del step **entero** es material de identidad: lo que
// decide qué hace un step es su directorio, no la lista de archivos que su autor
// se acordó de declarar en `templates:` (§5.3).
//
// Devuelve el PUERTO y no una huella ya calculada, y la diferencia importa: la
// regla `pipe-v1` esconde del árbol los archivos que ya entran normalizados, y
// esa exclusión es parte de la regla —tiene que estar en el dominio, donde una
// implementación independiente pueda leerla, y no en la fuente que sirve el
// árbol—.
//
// La firma es la de sus tres hermanos —`(ctx, pipelineLocalPath, step)`— para que
// la disposición del repositorio (`steps/<step>/`) siga siendo conocimiento de
// infraestructura y no se filtre a la cadena de pipeline.
type StepTreeRepository interface {
	Get(ctx *context.Context, pipelineLocalPath, step string) (fingerprint.TreeSource, error)
}
