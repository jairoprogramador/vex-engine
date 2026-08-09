package deployment

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// DeploymentIDVersion identifica la regla con la que se deriva la posición de un
// despliegue en la historia de su ambiente.
const DeploymentIDVersion = "dep-v1"

const (
	// deploymentIDHeader encabeza el material. Existe para que una identidad de
	// despliegue no pueda coincidir con el hash de otra cosa que se le parezca.
	deploymentIDHeader = "vex-deployment/" + DeploymentIDVersion

	deploymentIDKind = "deployment_id"
)

// DeploymentID responde «¿en qué posición de la historia de este ambiente cae
// esto?», que es una pregunta distinta de «¿qué se pretende ejecutar?».
//
// Se deriva del par (contenido, padre), que es la idea de git robada sin su
// motor (spec 17 §4, A7): dos ejecuciones del MISMO contenido al mismo ambiente
// producen `deployment_id` distintos, porque la segunda cuelga de la primera.
// De ahí sale la advertencia que gobierna todo el diseño: **la decisión de
// saltar un step no puede consultar `deployment_id`**, porque cambia siempre.
type DeploymentID struct {
	versionedHash
}

// DeploymentIDOf deriva la posición de un contenido dentro de un linaje.
//
// `parent` es el `deployment_id` que hasta ahora era la cabeza del linaje, y el
// valor cero significa «este es el primer despliegue de este ambiente». No es un
// hueco: la raíz aporta la cadena vacía al material, que ninguna identidad real
// puede producir —toda `DeploymentID` no cero lleva su prefijo—, así que el
// primer despliegue no puede colisionar con ninguno derivado.
//
// # Devuelve error, y el boceto de la spec no
//
// El boceto de §5.1 la escribe como total (`DeploymentIDOf(c, parent) DeploymentID`).
// Se implementa con error por la regla (d) de esa misma sección: **material
// incompleto ⇒ error, no identidad degradada**. Un `content_id` cero produciría
// una identidad perfectamente válida que colisiona con la de cualquier otro
// contenido que tampoco se pudo componer, y ese valor es permanente.
func DeploymentIDOf(content ContentID, parent DeploymentID) (DeploymentID, error) {
	if content.IsZero() {
		return DeploymentID{}, fmt.Errorf(
			"deployment: no se puede derivar un %s de un %s sin identidad",
			deploymentIDKind, contentIDKind)
	}

	sum := sha256.Sum256([]byte(canonicalDeploymentMaterial(content, parent)))
	value, err := newVersionedHash(deploymentIDKind, DeploymentIDVersion, hex.EncodeToString(sum[:]))
	if err != nil {
		return DeploymentID{}, err
	}
	return DeploymentID{versionedHash: value}, nil
}

// canonicalDeploymentMaterial es la forma normativa del material
// (`SPEC-CONTENT-v1.md` §6): tres líneas en orden fijo.
//
// Compone sobre las formas canónicas COMPLETAS —con su prefijo de versión—, no
// sobre los hashes pelados. Es la misma disciplina que la `cache_key`, y aquí
// pesa más porque el resultado es permanente: si compusiera sobre el hash
// pelado, un contenido identificado con la regla v1 y otro con la v2 darían el
// mismo despliegue.
func canonicalDeploymentMaterial(content ContentID, parent DeploymentID) string {
	return strings.Join([]string{
		deploymentIDHeader,
		strconv.Quote(content.String()),
		strconv.Quote(parent.String()),
	}, canonicalLineSep)
}

// ParseDeploymentID lee la representación canónica "dep-v1:<hash>".
func ParseDeploymentID(text string) (DeploymentID, error) {
	value, err := parseVersionedHash(deploymentIDKind, text)
	if err != nil {
		return DeploymentID{}, err
	}
	return DeploymentID{versionedHash: value}, nil
}

// Equals es la regla de igualdad del value object.
func (id DeploymentID) Equals(other DeploymentID) bool {
	return id.equals(other.versionedHash)
}
