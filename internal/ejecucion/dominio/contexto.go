package dominio

import (
	"crypto/sha256"
	"encoding/hex"
)

// ContextoDeEjecucion es una cadena opaca que quien invoca declara para decir «en qué entorno externo corre
// este intento». El motor no la interpreta: solo la mezcla en la huella de variables, de modo que cambiarla
// invalide la reutilización de los pasos que miran variables. El valor cero es «sin contexto».
//
// No es una variable del pipeline ni un secreto, y no se guarda: al Historial solo llega la huella combinada.
type ContextoDeEjecucion struct{ valor string }

const (
	longitudMaximaDelContexto = 128

	// prefijoHashConContexto distingue una huella de variables mezclada con un contexto de la que da Resolución.
	prefijoHashConContexto = "entradas-v2+contexto:"
)

// NuevoContexto comprueba que el contexto sea corto y de un conjunto cerrado de caracteres: es un dato de quien
// invoca que acaba dentro de un hash, y nada más. El error dice el campo, nunca el valor.
func NuevoContexto(valor string) (ContextoDeEjecucion, error) {
	if len(valor) > longitudMaximaDelContexto {
		return ContextoDeEjecucion{}, invalidoElCampo("Contexto", "", "el contexto no puede pasar de %d bytes", longitudMaximaDelContexto)
	}
	for i := 0; i < len(valor); i++ {
		if !esCaracterDeContexto(valor[i]) {
			return ContextoDeEjecucion{}, invalidoElCampo("Contexto", "", "el contexto solo admite letras, números y . _ : / @ + = -")
		}
	}
	return ContextoDeEjecucion{valor: valor}, nil
}

func esCaracterDeContexto(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '.', '_', ':', '/', '@', '+', '=', '-':
		return true
	}
	return false
}

// ConLasVariables devuelve la huella de variables que el paso debe comparar. Sin contexto es la misma que dio
// Resolución, byte a byte; con contexto, un resumen de la huella y del contexto.
func (c ContextoDeEjecucion) ConLasVariables(huella HashDeVariables) HashDeVariables {
	if c.valor == "" {
		return huella
	}
	suma := sha256.Sum256([]byte(huella.String() + "\x00contexto\x00" + c.valor))
	return NuevoHashDeVariables(prefijoHashConContexto + hex.EncodeToString(suma[:]))
}
