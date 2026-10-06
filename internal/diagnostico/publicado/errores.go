package publicado

import "errors"

// ErrInvalido: lo pedido no es válido, como un ambiente o un intento vacíos.
var ErrInvalido = errors.New("diagnóstico: inválido")
