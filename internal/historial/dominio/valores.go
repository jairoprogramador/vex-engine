package dominio

import "fmt"

// IdIntento, IdDespliegue e IdLanzamiento son identidades propias y únicas. Las da el puerto Identidades.
type (
	IdIntento     string
	IdDespliegue  string
	IdLanzamiento string
)

// Ambiente es la separación en la que ocurre un intento: dev, staging, producción.
type Ambiente string

// NombrePaso identifica un paso por su nombre.
type NombrePaso string

// NombreVariable es una clave que no es forma propia: la pidió Resolución de Variables para pedir de vuelta
// el valor de la última vez de cada variable (IT-07 DEC-07.6).
type NombreVariable string

// Solicitante es quien pidió un intento.
type Solicitante string

// HashDelCodigo es una clave que no es forma propia: la pidió Diagnóstico para buscar el último despliegue
// de un ambiente con un código dado (IT-07 DEC-07.6). El Historial no sabe qué significa: solo que dos
// intentos tienen la misma.
type HashDelCodigo string

// Contexto nombra al contexto dueño de un contenido.
type Contexto string

// Contenido es lo que dice un registro. Es opaco: el Historial lo guarda sin aplicarle ninguna regla, y lo
// interpreta quien lo produjo (IT-03 DEC-03.13).
type Contenido struct {
	Contexto Contexto
	Datos    []byte
}

// Estado es cómo terminó un intento. Sin desenlace y abandonado no son estados.
type Estado int

const (
	Exitoso Estado = iota + 1
	Fallido
	Cancelado
)

func (e Estado) String() string {
	switch e {
	case Exitoso:
		return "exitoso"
	case Fallido:
		return "fallido"
	case Cancelado:
		return "cancelado"
	}
	return fmt.Sprintf("estado(%d)", int(e))
}

func (e Estado) valido() bool {
	return e == Exitoso || e == Fallido || e == Cancelado
}

// Ambito es qué variables ve un paso, y por eso la posición bajo la que se buscan sus registros: su
// ambiente o compartido entre ambientes (IT-06 DEC-06.12).
type Ambito struct {
	Compartido bool
	Ambiente   Ambiente // vacío si es compartido
}

// AmbitoDeAmbiente es el ámbito de un paso que guarda su propia historia en cada ambiente.
func AmbitoDeAmbiente(a Ambiente) Ambito { return Ambito{Ambiente: a} }

// AmbitoCompartido es el ámbito de un paso cuya historia es la misma en todos los ambientes.
func AmbitoCompartido() Ambito { return Ambito{Compartido: true} }

// PasoDeclarado es un paso del pipeline tal como lo declara la apertura de un intento.
type PasoDeclarado struct {
	Nombre     NombrePaso
	Compartido bool
}

// PasoEnSuAmbito es la posición bajo la que se buscan los registros de un paso.
type PasoEnSuAmbito struct {
	Paso   NombrePaso
	Ambito Ambito
}

// Evidencia es el enlace al registro con el que se hizo de verdad un paso que no se re-ejecutó: el
// intento y el paso de ese registro.
type Evidencia struct {
	Intento IdIntento
	Paso    NombrePaso
}
