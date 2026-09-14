package record

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

const (
	// DigestVersion encabeza el resumen para que la convención pueda cambiar sin
	// que dos formas distintas se confundan, igual que hacen las tres reglas de
	// huella y el `deployment_id`.
	//
	// Sube de `sha256` a `hmac-sha256` con la spec 20, y el prefijo es lo que hace
	// que los resúmenes emitidos hasta aquí no mientan: un `sha256:…` y un
	// `hmac-sha256:…` del mismo valor no se parecen, y un consumidor que compare
	// dos ejecuciones de distinta época ve que no son comparables en vez de
	// concluir que el valor cambió.
	DigestVersion = "hmac-sha256"

	// digestKeyPurpose separa esta derivación de cualquier otro uso del mismo
	// secreto. Va dentro del material de la clave, no al lado: sin él, un segundo
	// consumidor del secreto —una firma, un identificador— derivaría la misma
	// clave para el mismo proyecto y los dos usos se contaminarían.
	digestKeyPurpose = "vex-parameter-digest/v1"

	// minSecretLen es el mínimo del secreto local. No es una preferencia: un
	// secreto corto hace la clave adivinable, y con la clave adivinada el digest
	// vuelve a ser lo que esta spec quita de en medio —una búsqueda en una tabla—.
	minSecretLen = 32
)

// Digester resume el valor con el que se resolvió un parámetro.
//
// Es un puerto y no una función porque el resumen dejó de ser puro: depende de
// una clave que sólo se conoce cuando se sabe DE QUÉ PROYECTO es la ejecución, y
// el adaptador de hechos se cablea antes de leer el `RequestInput`. Quien emite
// no tiene que saber nada de eso.
type Digester interface {
	// Digest devuelve el resumen en su forma externa, con prefijo de versión.
	//
	// Devuelve error —y no un resumen degradado— cuando todavía no hay clave: un
	// digest sin sal es exactamente el defecto que la spec 20 §2 describe, y
	// emitirlo «por no fallar» lo reintroduciría en silencio justo en el hecho que
	// existe para no llevar el valor.
	Digest(value string) (string, error)
}

// ParameterDigester es el resumidor del proceso: nace con el secreto local y
// recibe el proyecto cuando el `RequestInput` se lee.
//
// # Por qué HMAC y no SHA-256, y por qué sólo aquí (spec 20 §5.2)
//
// «¿Hash o HMAC?» eran dos preguntas con respuestas opuestas, y tratarlas como
// una sola es lo que dejó la decisión abierta tanto tiempo:
//
//   - `content_id` resume TODO el contenido —dos árboles enteros y la
//     declaración de cada parámetro—, así que invertirlo exigiría conocer ya
//     todo lo demás. Va SIN SAL y debe ir sin sal: que dos organizaciones con el
//     mismo contenido obtengan la misma identidad es su razón de existir.
//   - Esto resume UN valor suelto. `sha256("3")` no es un secreto, es una
//     búsqueda en una tabla, y publicarlo contradice literalmente la promesa de
//     «solo nombres y hashes». La comparabilidad entre organizaciones no aplica:
//     este resumen sirve para comparar un proyecto contra su propia historia
//     —«`DB_POOL_SIZE` cambió»— y para eso basta con ser estable dentro del
//     proyecto.
//
// # El vacío también tiene resumen
//
// «Se resolvió a la cadena vacía» es un hecho distinto de «no se resolvió», y
// colapsarlos en la ausencia de digest perdería justo el caso que suele ser el
// defecto (spec 03 §5.3). Ojo al leerlo: `hmac("")` es una constante por
// proyecto y perfectamente reconocible, así que ese digest no protege nada —no
// hay nada que proteger—. Un output EXTRAÍDO nunca llega vacío (`CommandVariable`
// lo rechaza), así que esa constante sólo puede venir de una declaración.
type ParameterDigester struct {
	secret []byte

	// La clave llega en un segundo momento —el proyecto se conoce al leer el
	// input, y el adaptador de hechos se cablea antes—, igual que la tira de
	// hechos llega al emisor en `Open`. El mutex no es por concurrencia de
	// escritura sino porque quien la instala y quien la lee son dos momentos
	// distintos del mismo proceso.
	mu  sync.Mutex
	key []byte
}

var _ Digester = (*ParameterDigester)(nil)

// NewParameterDigester construye el resumidor sin clave. El secreto es el local
// de la instalación; la clave por proyecto se deriva de él en `Bind`.
func NewParameterDigester(secret []byte) (*ParameterDigester, error) {
	if len(secret) < minSecretLen {
		return nil, fmt.Errorf(
			"record: el secreto del resumen tiene %d bytes y necesita al menos %d",
			len(secret), minSecretLen)
	}
	copia := make([]byte, len(secret))
	copy(copia, secret)
	return &ParameterDigester{secret: copia}, nil
}

// Bind deriva la clave de este proyecto.
//
// La derivación entra el PROPÓSITO y el identificador del proyecto, en ese
// orden y separados por un byte nulo: el identificador es un UUID y no puede
// contener el separador, así que el material es inyectivo y dos proyectos no
// pueden derivar la misma clave.
//
// Enlazar dos veces es un error del cableado y no un cambio de proyecto: una
// ejecución es de un proyecto, y permitir el cambio a mitad dejaría dos digests
// del mismo valor en la misma tira.
func (d *ParameterDigester) Bind(projectID string) error {
	if projectID == "" {
		return fmt.Errorf("record: no se puede derivar la clave del resumen sin proyecto")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.key != nil {
		return fmt.Errorf("record: la clave del resumen ya estaba derivada")
	}

	mac := hmac.New(sha256.New, d.secret)
	mac.Write([]byte(digestKeyPurpose))
	mac.Write([]byte{0})
	mac.Write([]byte(projectID))
	d.key = mac.Sum(nil)
	return nil
}

// Digest resume el valor con la clave del proyecto.
func (d *ParameterDigester) Digest(value string) (string, error) {
	d.mu.Lock()
	key := d.key
	d.mu.Unlock()

	if key == nil {
		return "", fmt.Errorf(
			"record: no hay clave con la que resumir el valor de un parámetro" +
				" (¿se emitió un hecho antes de saber de qué proyecto es la ejecución?)")
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(value))
	return DigestVersion + ":" + hex.EncodeToString(mac.Sum(nil)), nil
}
