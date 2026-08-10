package record

import (
	"crypto/sha256"
	"encoding/hex"
)

// digestVersion encabeza el resumen para que la convención pueda cambiar sin
// que dos formas distintas se confundan, igual que hacen las tres reglas de
// huella y el `deployment_id`.
const digestVersion = "sha256"

// DigestOf resume el valor con el que se resolvió un parámetro.
//
// # Por qué el valor no entra y su resumen sí
//
// Es la otra mitad de la regla de la spec 14 —en la IDENTIDAD entra la
// declaración, nunca el valor— aplicada al REGISTRO: el valor resuelto es un
// hecho y se registra, pero resumido, porque un registro que viaja fuera de la
// organización no puede llevar secretos en claro. Un `parameter_resolved` con el
// valor entero convertiría el registro en un vector de fuga, y uno sin nada del
// valor no dejaría responder «¿corrió con lo mismo que ayer?», que es para lo
// que existe.
//
// # La convención es de la spec 20, y ésta es la provisional
//
// Quien decide qué se redacta y cómo es la spec 20. Hasta entonces se emite un
// SHA-256 con su prefijo: es de una sola dirección —así que no reintroduce el
// valor por la puerta de atrás— y es comparable entre ejecuciones, que es lo
// único que el consumidor necesita hoy. El prefijo es lo que permite que la 20
// cambie de convención sin que los resúmenes viejos mientan.
//
// El valor vacío también tiene resumen: «se resolvió a la cadena vacía» es un
// hecho distinto de «no se resolvió», y colapsarlos en la ausencia de digest
// perdería justo el caso que suele ser el defecto.
func DigestOf(value string) string {
	sum := sha256.Sum256([]byte(value))
	return digestVersion + ":" + hex.EncodeToString(sum[:])
}
