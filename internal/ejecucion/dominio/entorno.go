package dominio

import (
	"slices"
	"strings"
)

// Entorno son las variables de entorno que quien invoca pide para los comandos de un intento: nombre y valor.
//
// No son variables del pipeline: no pasan por Resolución, no entran en ningún hash ni se guardan en el Historial
// (DEC-04.7: el motor no guarda valores de variables). Tampoco son secretos para el motor, que no los gestiona
// (DEC-08.8, es otro producto): son variables que pueden o no ser sensibles, y que lo sean es cosa de quien las pasa.
// Lo único que el dominio hace por ellas es no contarlas sin querer: al imprimir un Entorno solo salen sus nombres.
//
// Es un valor inmutable. El valor cero es un entorno sin variables.
type Entorno struct {
	variables map[string]string
}

// NuevoEntorno comprueba que cada nombre sea el de una variable de entorno y que ningún valor tenga un NUL, que el
// sistema operativo no puede pasar. Copia las variables: cambiar el mapa después no cambia el Entorno. Los errores
// dicen el nombre de la variable, nunca su valor.
func NuevoEntorno(variables map[string]string) (Entorno, error) {
	for nombre, valor := range variables {
		if !esNombreDeVariableDeEntorno(nombre) {
			return Entorno{}, invalidoElCampo("Entorno", nombre, "el nombre de variable de entorno %q no vale: letras, números y _, sin empezar por un número", nombre)
		}
		if strings.ContainsRune(valor, 0) {
			return Entorno{}, invalidoElCampo("Entorno", nombre, "el valor de la variable de entorno %q tiene un carácter NUL", nombre)
		}
	}
	if len(variables) == 0 {
		return Entorno{}, nil
	}
	copia := make(map[string]string, len(variables))
	for nombre, valor := range variables {
		copia[nombre] = valor
	}
	return Entorno{variables: copia}, nil
}

func esNombreDeVariableDeEntorno(nombre string) bool {
	if nombre == "" {
		return false
	}
	for i := 0; i < len(nombre); i++ {
		c := nombre[i]
		letra := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_'
		if !letra && !(i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// Vacio dice si no hay ninguna variable.
func (e Entorno) Vacio() bool { return len(e.variables) == 0 }

// Lista son las variables como NOMBRE=valor, ordenadas por nombre: el mismo entorno da siempre la misma lista.
func (e Entorno) Lista() []string {
	nombres := e.nombres()
	lista := make([]string, 0, len(nombres))
	for _, nombre := range nombres {
		lista = append(lista, nombre+"="+e.variables[nombre])
	}
	return lista
}

func (e Entorno) nombres() []string {
	nombres := make([]string, 0, len(e.variables))
	for nombre := range e.variables {
		nombres = append(nombres, nombre)
	}
	slices.Sort(nombres)
	return nombres
}

// String solo dice los nombres: un Entorno que acaba en un error o en un log no cuenta sus valores.
func (e Entorno) String() string {
	return "entorno[" + strings.Join(e.nombres(), ",") + "]"
}

// GoString hace lo mismo con %#v.
func (e Entorno) GoString() string { return e.String() }
