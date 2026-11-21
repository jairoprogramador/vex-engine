package dominio

import (
	"regexp"
	"strings"
)

// Redactado es lo que queda en lugar de lo que tiene forma de secreto.
const Redactado = "[REDACTADO]"

// formaDeJWT son tres segmentos de base64url separados por puntos, el primero empezando por eyJ (`{"` en
// base64). Sin ancla de palabra al principio, a propósito: un token pegado a lo que tenga delante (un código de
// color del terminal, `%3D`) también tiene que caer, aunque eso tape de más alguna vez («keyJoin.a.b»).
var formaDeJWT = regexp.MustCompile(`eyJ[\w-]+\.[\w-]+\.[\w-]+`)

// RedactarJWT reemplaza cada cadena con forma de JWT por Redactado. Es de mejor esfuerzo (AJ-21): no detecta
// el token codificado de otra forma, ni uno partido por un salto de línea antes de su segundo punto. RE2 es de
// tiempo lineal: no hay entradas que la cuelguen.
func RedactarJWT(texto string) string { return formaDeJWT.ReplaceAllString(texto, Redactado) }

// Las demás formas que se redactan: pocas, baratas y con un prefijo que el texto normal no trae. En las que
// tienen nombre (`AccountKey=`), el nombre se queda y solo se tapa el valor: el log sigue diciendo qué había ahí.
var (
	// Una clave privada en PEM, del BEGIN al END —o hasta el final, si la salida se cortó antes—. Un certificado
	// o una clave pública no son secretos y no se tocan.
	formaDeClavePrivada = regexp.MustCompile(
		`-----BEGIN (?:[A-Z0-9]+ )*PRIVATE KEY-----[\s\S]*?(?:-----END (?:[A-Z0-9]+ )*PRIVATE KEY-----|\z)`)

	// La clave de una cadena de conexión de Azure: AccountKey= (Storage) y SharedAccessKey= (Service Bus, Event
	// Hubs), en base64.
	formaDeClaveDeCuenta = regexp.MustCompile(`(?i)\b((?:Account|SharedAccess)Key=)[A-Za-z0-9+/]{16,}={0,2}`)

	// Una SAS dentro de una cadena de conexión: todo su valor, hasta el ; o el espacio.
	formaDeSASEnCadena = regexp.MustCompile(`(?i)\b(SharedAccessSignature=)[^\s;"']{8,}`)

	// La firma de una SAS en una URL o en una cadena de consulta: sin ella, el resto no sirve.
	formaDeFirmaDeSAS = regexp.MustCompile(`([?&]sig=)[A-Za-z0-9%+/=_-]{16,}`)

	// Lo que sigue a «Bearer». Solo se tapa si además tiene algún número (esTokenOpaco): «Bearer» seguido de una
	// palabra larga es texto normal.
	formaDeBearer = regexp.MustCompile(`\b([Bb]earer\s+)([A-Za-z0-9._~+/-]{20,}=*)`)
)

// Redactar reemplaza por Redactado lo que tiene forma de secreto conocido: un JWT (RedactarJWT), una clave
// privada en PEM, la clave o la SAS de una cadena de conexión de Azure (`AccountKey=`, `SharedAccessKey=`,
// `SharedAccessSignature=`), la firma de una SAS en una URL (`?sig=`, `&sig=`) y el token que sigue a `Bearer`.
//
// Es de mejor esfuerzo, no una garantía (AJ-21): solo conoce estas formas. No detecta un secreto codificado de
// otra manera (base64, hexadecimal), partido entre líneas o sin prefijo —una contraseña suelta—, y lo que un
// comando captura con probe se guarda con su valor real. Es idempotente, y de tiempo lineal sobre el texto.
func Redactar(texto string) string {
	texto = RedactarJWT(texto)
	texto = formaDeClavePrivada.ReplaceAllString(texto, Redactado)
	texto = formaDeClaveDeCuenta.ReplaceAllString(texto, "${1}"+Redactado)
	texto = formaDeSASEnCadena.ReplaceAllString(texto, "${1}"+Redactado)
	texto = formaDeFirmaDeSAS.ReplaceAllString(texto, "${1}"+Redactado)
	return formaDeBearer.ReplaceAllStringFunc(texto, redactarBearer)
}

func redactarBearer(coincidencia string) string {
	partes := formaDeBearer.FindStringSubmatch(coincidencia)
	if !esTokenOpaco(partes[2]) {
		return coincidencia
	}
	return partes[1] + Redactado
}

// esTokenOpaco: un token de verdad, con veinte caracteres o más, casi siempre trae algún número; una palabra, no.
func esTokenOpaco(candidato string) bool { return strings.ContainsAny(candidato, "0123456789") }
