package infraestructura

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Almacen es lo que el Historial necesita de donde guarda: secuencias de registros que solo crecen. Cada
// agregado es una secuencia, y es aquí donde viven las tres garantías de IT-10 DEC-10.3:
//
//  1. lo que Anadir da por escrito se puede leer antes de seguir;
//  2. un registro no se sobrescribe nunca;
//  3. la escritura es condicional: Anadir en una posición que ya existe devuelve dominio.ErrConflicto.
//
// Lo que se compre (RD-11) implementa esta interfaz.
type Almacen interface {
	// Leer devuelve los registros de una secuencia en orden. Una secuencia que no existe está vacía.
	Leer(ctx context.Context, s Secuencia) ([][]byte, error)
	// Anadir escribe un registro en una posición, que es la siguiente a la última. Si la posición ya existe,
	// devuelve dominio.ErrConflicto y no escribe nada.
	Anadir(ctx context.Context, s Secuencia, posicion int, registro []byte) error
	// Nombres son los de las secuencias de una familia.
	Nombres(ctx context.Context, familia string) ([]string, error)
}

// Secuencia se nombra por su familia y, si la familia tiene varias, por su nombre: «intentos» e id,
// «despliegues» y ambiente, «salidas» e intento. «lanzamientos» es una sola, sin nombre.
type Secuencia struct {
	Familia string
	Nombre  string
}

func (s Secuencia) String() string {
	if s.Nombre == "" {
		return s.Familia
	}
	return s.Familia + "/" + s.Nombre
}

// Familias de secuencias.
const (
	familiaIntentos     = "intentos"
	familiaDespliegues  = "despliegues"
	familiaOcupaciones  = "ocupaciones"
	familiaReservas     = "reservas"
	familiaLanzamientos = "lanzamientos"
	familiaSalidas      = "salidas"
)

// segmento convierte un nombre en un segmento de ruta seguro en cualquier sistema: letras y dígitos ASCII,
// «-» y «_» quedan igual, y el resto se escribe como %XX. Así, un nombre que viene de fuera, como «../x»,
// nunca sale de su directorio.
func segmento(nombre string) (string, error) {
	if nombre == "" {
		return "", fmt.Errorf("almacén: un nombre de secuencia no puede estar vacío")
	}
	var b strings.Builder
	for _, c := range []byte(nombre) {
		if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9') || c == '-' || c == '_' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String(), nil
}

// nombreDeSegmento deshace segmento.
func nombreDeSegmento(s string) (string, error) {
	var b strings.Builder
	for k := 0; k < len(s); k++ {
		if s[k] != '%' {
			b.WriteByte(s[k])
			continue
		}
		if k+2 >= len(s) {
			return "", fmt.Errorf("almacén: segmento mal escrito: %q", s)
		}
		c, err := strconv.ParseUint(s[k+1:k+3], 16, 8)
		if err != nil {
			return "", fmt.Errorf("almacén: segmento mal escrito %q: %w", s, err)
		}
		b.WriteByte(byte(c))
		k += 2
	}
	return b.String(), nil
}
