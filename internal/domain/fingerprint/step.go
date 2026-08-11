package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
)

// StepVersion identifica la regla de la huella de un step: `sf-v1`.
//
// Es la que sustituye a `cache.CacheKey` (`ck-v1`) como respuesta a «¿es el
// mismo trabajo?». La diferencia con aquélla no es de cálculo sino de material:
// `ck-v1` hasheaba la DIRECCIÓN —sujeto, pipeline, ámbito y step— junto con el
// contenido, así que el mismo trabajo guardado en dos sitios producía dos
// identidades. Aquí sólo entra el contenido, y la dirección es la clave de
// estado (spec 11 §5.1).
//
// La sustitución no necesita migrador ni código de compatibilidad: la
// comparación es de cadenas COMPLETAS con prefijo (`state.StepRecord.Revives`),
// así que ningún registro escrito con `ck-v1:` puede revivir contra una huella
// `sf-v1:`. El primer efecto de la spec 27 es una re-ejecución de todos los
// steps, que es el efecto correcto de cambiar la regla de identidad.
const StepVersion = "sf-v1"

// stepHeader encabeza el material. Existe para que esta huella no pueda
// coincidir con el hash de otra cosa que se le parezca.
const stepHeader = "vex-step-fingerprint/" + StepVersion

// ComputeStepFingerprint compone la huella de un step: su declaración
// (`pipe-v1`) y, SI EL STEP LO DECLARA, la huella del árbol del proyecto (`v1`).
//
// La regla completa y normativa está en SPEC-STEP-v1.md.
//
// # Los dos términos entran por su forma canónica COMPLETA
//
// Nunca por el hash pelado. De ahí sale que subir `pipe-v1` a `v2` invalide
// todas las `sf-v1` emitidas **sin una línea de código extra**, que es el OCP
// que la spec 08 §5.2' prometía y que sólo se conserva mientras la composición
// sea sobre `String()`. Por lo mismo, esta regla NO comprueba la versión de sus
// términos: hacerlo obligaría a tocarla para aceptar un `pipe-v2`, y con ello la
// propiedad se perdería.
//
// # Por qué la exclusión va en un booleano NEGATIVO
//
// Porque su valor cero es el SEGURO. Un `includesProject bool` dejaría que
// olvidarlo produjera una huella sin el código del proyecto —un step que deja de
// re-ejecutarse ante un cambio de código, la peor omisión posible— mientras que
// olvidar éste produce una re-ejecución de más. Y el hueco tiene que ir
// DECLARADO por los dos lados: excluir el proyecto y a la vez traerlo también es
// un error, porque si no la forma canónica de un mismo step dependería de si
// alguien se acordó de limpiar el campo.
//
// # Material incompleto ⇒ error, nunca huella degradada
//
// Una huella con un hueco es válida y COLISIONA con la de cualquier material al
// que le falte lo mismo, y esa colisión se manifiesta como un step que revive
// sin haberse ejecutado jamás. Aguas arriba, «no se pudo componer la huella»
// significa ejecutar el step y escribir su registro SIN huella, que es un
// registro que no revive nunca: el fail-open se conserva por construcción
// (spec 10 §9.5).
func ComputeStepFingerprint(
	declaration Fingerprint, project Fingerprint, projectExcluded bool) (Fingerprint, error) {

	if declaration.IsZero() {
		return Fingerprint{}, errors.New(
			"fingerprint: la huella del step no tiene la declaración del step")
	}

	switch {
	case projectExcluded && !project.IsZero():
		return Fingerprint{}, errors.New(
			"fingerprint: la huella del step excluye la del proyecto y a la vez la trae")
	case !projectExcluded && project.IsZero():
		return Fingerprint{}, errors.New(
			"fingerprint: la huella del step no tiene la del árbol del proyecto")
	}

	material := strings.Join([]string{
		stepHeader,
		strconv.Quote(declaration.String()),
		// El término CONDICIONAL. Excluido es `Q("")`, que no es producible por
		// una huella real —ninguna forma canónica es la cadena vacía—, así que la
		// ampliación del dominio es inyectiva: ninguna huella con proyecto puede
		// coincidir con una sin él.
		strconv.Quote(project.String()),
	}, entrySeparator)

	sum := sha256.Sum256([]byte(material))
	return newVersioned(StepVersion, hex.EncodeToString(sum[:]))
}
