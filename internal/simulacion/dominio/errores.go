package dominio

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalido: lo que se pide construir o simular no es válido, como un ámbito de ambiente sin nombre.
	ErrInvalido = errors.New("simulacion: inválido")
)

func invalido(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrInvalido}, args...)...)
}

// ParametroInvalidoError es un ErrInvalido que dice qué campo de la petición y qué valor no valen, para que quien
// presenta el error no tenga que leer el texto. Solo lo llevan los valores que da quien invoca: las invariantes de
// lo que viene del pipeline no son un campo de la petición y siguen siendo un invalido() sin campo.
type ParametroInvalidoError struct {
	Campo  string
	Valor  string
	Motivo string
}

func (e *ParametroInvalidoError) Error() string { return ErrInvalido.Error() + ": " + e.Motivo }

func (e *ParametroInvalidoError) Is(destino error) bool { return destino == ErrInvalido }

func (e *ParametroInvalidoError) ParametroInvalido() (campo, valor string) { return e.Campo, e.Valor }

// NuevoParametroInvalido es el error de un valor de la petición que no vale y que se descubre fuera del dominio,
// en la aplicación o en un adaptador: dice el campo y el valor.
func NuevoParametroInvalido(campo, valor, motivo string) error {
	return &ParametroInvalidoError{Campo: campo, Valor: valor, Motivo: motivo}
}

func invalidoElCampo(campo, valor, formato string, args ...any) error {
	return &ParametroInvalidoError{Campo: campo, Valor: valor, Motivo: fmt.Sprintf(formato, args...)}
}
