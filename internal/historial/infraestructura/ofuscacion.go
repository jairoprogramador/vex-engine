package infraestructura

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// Ofuscar no es cifrar (IT-04 DEC-04.7): cualquiera con este código lo revierte. Solo evita que un valor se
// lea a simple vista en el almacén, o que aparezca al buscarlo en sus ficheros. Proteger secretos es asunto
// de otro producto (IT-08 DEC-08.8). No hay clave por máquina: el valor tiene que volver en cualquiera.

const prefijoOfuscado = "ofuscado-v1:"

var mascaraDeOfuscacion = []byte("vex-historial")

func ofuscar(valor string) string {
	return prefijoOfuscado + base64.RawStdEncoding.EncodeToString(enmascarar([]byte(valor)))
}

func desofuscar(ofuscado string) (string, error) {
	codificado, ok := strings.CutPrefix(ofuscado, prefijoOfuscado)
	if !ok {
		return "", fmt.Errorf("almacén: valor ofuscado con un formato desconocido")
	}
	datos, err := base64.RawStdEncoding.DecodeString(codificado)
	if err != nil {
		return "", fmt.Errorf("almacén: valor ofuscado ilegible: %w", err)
	}
	return string(enmascarar(datos)), nil
}

func enmascarar(datos []byte) []byte {
	resultado := make([]byte, len(datos))
	for k, b := range datos {
		resultado[k] = b ^ mascaraDeOfuscacion[k%len(mascaraDeOfuscacion)]
	}
	return resultado
}
