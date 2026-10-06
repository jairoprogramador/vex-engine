package publicado

import "errors"

// ErrInvalido: lo pedido no es válido, como un ambiente o un despliegue vacíos.
var ErrInvalido = errors.New("lanzamiento: inválido")
