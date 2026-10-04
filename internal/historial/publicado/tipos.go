package publicado

import "time"

// Nada de lo que se publica aquí lleva el valor de una variable (IT-04 DEC-04.7). El valor solo sale por la
// relación reservada, hacia Resolución de Variables.

// Estado es cómo terminó un intento.
type Estado string

const (
	Exitoso   Estado = "exitoso"
	Fallido   Estado = "fallido"
	Cancelado Estado = "cancelado"
)

// Contenido es lo que dice un registro, con el contexto al que pertenece. El Historial no lo interpreta.
type Contenido struct {
	Contexto string
	Datos    []byte
}

// Ambito es la posición bajo la que se buscan los registros de un paso: su ambiente o compartido.
type Ambito struct {
	Compartido bool
	Ambiente   string // vacío si es compartido
}

// PasoDeclarado es un paso del pipeline, con su ámbito.
type PasoDeclarado struct {
	Nombre     string
	Compartido bool
}

// Evidencia es el enlace al registro con el que se hizo de verdad un paso que no se re-ejecutó.
type Evidencia struct {
	Intento string
	Paso    string
}

// Apertura es lo que declara un intento al abrirse.
type Apertura struct {
	Ambiente    string
	Solicitante string
	// Pasos son todos los del pipeline, en orden.
	Pasos     []PasoDeclarado
	HastaPaso string
	// ConCommits es falso si el material es una copia de trabajo: el intento nunca llega a despliegue.
	ConCommits    bool
	HashDelCodigo string
	// Contenido lleva, por ejemplo, el commit de cada fuente y el orden de los ambientes.
	Contenido Contenido
}

// TipoDeRegistroDePaso dice qué registró un paso.
type TipoDeRegistroDePaso string

const (
	Comienzo      TipoDeRegistroDePaso = "comienzo"
	Final         TipoDeRegistroDePaso = "final"
	NoReejecucion TipoDeRegistroDePaso = "no_reejecucion"
)

// RegistroDePaso es un comienzo, un final o una no re-ejecución de un paso en un intento.
type RegistroDePaso struct {
	Intento   string
	Paso      string
	Tipo      TipoDeRegistroDePaso
	Exitoso   bool      // solo en un final
	Evidencia Evidencia // solo en una no re-ejecución
	Instante  time.Time
	Contenido Contenido
}

// Variable es el registro de una variable de un paso: su hash y su origen van en el contenido, que es de
// Resolución de Variables. No lleva valor.
type Variable struct {
	Intento   string
	Paso      string
	Nombre    string
	Instante  time.Time
	Contenido Contenido
}

// Intento es un intento del historial: su apertura, lo que registraron sus pasos y cómo terminó.
type Intento struct {
	Id        string
	Apertura  Apertura
	Instante  time.Time // el de la apertura
	Registros []RegistroDePaso
	// Estado está vacío si el intento no tiene desenlace.
	Estado     Estado
	Destino    string // el despliegue al que volvió, si fue un rollback
	Abandonado bool
}

// SinDesenlace: ningún registro dice cómo terminó. No es un estado.
func (i Intento) SinDesenlace() bool { return i.Estado == "" }

// Salida es lo que escribió un comando de un paso al terminar, y si salió bien. No se confunde con los
// registros de un paso: se guarda aparte, para poder consultarla por intento.
type Salida struct {
	Paso     string
	Comando  string
	Exitoso  bool
	Texto    string
	Instante time.Time
}

// FiltroDeSalidas dice qué salidas de un intento se piden. El cero es todas.
type FiltroDeSalidas int

const (
	TodasLasSalidas FiltroDeSalidas = iota
	SoloLasExitosas
	SoloLasFallidas
)

// Admite dice si una salida con ese resultado entra en lo que el filtro pide.
func (f FiltroDeSalidas) Admite(exitoso bool) bool {
	switch f {
	case SoloLasExitosas:
		return exitoso
	case SoloLasFallidas:
		return !exitoso
	}
	return true
}

// Despliegue es un intento exitoso de todos los pasos de un pipeline.
type Despliegue struct {
	Id       string
	Ambiente string
	Intento  string
	Padre    string // vacío si es el primero de su ambiente
	Instante time.Time
}

// Lanzamiento es un despliegue que se hace visible. Su versión y su nombre van en el contenido.
type Lanzamiento struct {
	Id         string
	Ambiente   string
	Despliegue string
	Instante   time.Time
	Contenido  Contenido
}

// Reserva: el dueño del negocio reservó el ambiente o lo liberó.
type Reserva struct {
	Ambiente  string
	Reservado bool
	Instante  time.Time
}

// DespliegueRegistrado anuncia un despliegue nuevo, después de escribirlo.
type DespliegueRegistrado struct {
	Despliegue Despliegue
}
