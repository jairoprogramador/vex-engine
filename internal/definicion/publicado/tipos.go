package publicado

import "time"

// Pipeline es el pipeline comprobado: nadie recibe uno que no haya pasado la comprobación (IT-08 DEC-08.2). Es
// el lenguaje que escribe el DevOps, y se lee con la versión de su formato.
type Pipeline struct {
	// Version es la del formato con el que se escribió.
	Version string
	// Commit es el del pipeline, y está vacío si no lo hay.
	Commit string
	// Hash es el del pipeline.
	Hash      string
	Ambientes []Ambiente // en su orden
	Pasos     []Paso     // en su orden
	// Variables son las variables declaradas, cada una con su ámbito.
	Variables []VariableDeclarada
}

// Ambiente es un ambiente en su lugar del orden.
type Ambiente struct {
	Nombre      string
	Descripcion string
	// Valor es con el que se nombra el ambiente en variables/.
	Valor string
}

// Paso se identifica por su nombre. Orden es solo su lugar: renumerarlo no cambia qué paso es.
type Paso struct {
	Nombre string
	Orden  int
	// Reglas son las cosas que mira para decidir si se re-ejecuta.
	Reglas []Regla
	// EdadMaxima es cero si el paso no caduca.
	EdadMaxima time.Duration
	Comandos   []Comando
	// Material, junto con los comandos, son las instrucciones del paso. No incluye rules.yaml.
	Material []Fichero
	// Compartido es su propio ámbito (rules.yaml, scope: shared): decide qué ve el paso (solo lo compartido,
	// no lo de un ambiente), bajo qué ámbito archiva su historia, y qué hereda por defecto una variable de
	// salida sin scope propio. Si no, su ámbito es el que representa al ambiente en que se ejecuta, y ve ese
	// ámbito más el compartido.
	Compartido bool
}

// Regla es una cosa que un paso mira para decidir si se re-ejecuta.
type Regla string

const (
	ReglaCodigo        Regla = "codigo"
	ReglaInstrucciones Regla = "instrucciones"
	ReglaVariables     Regla = "variables"
)

type Comando struct {
	Nombre      string
	Descripcion string
	Linea       string
	// Directorio es el workdir escrito, limpio y relativo al directorio del paso.
	Directorio string
	// Plantillas son las rutas del material que se interpolan, relativas al directorio del paso.
	Plantillas []string
	Salidas    []VariableDeSalida
	// Aserciones son las que su salida tiene que cumplir para que el comando se dé por bueno.
	Aserciones []Asercion
}

// VariableDeSalida es un nombre y la expresión regular que dice qué forma tendrá su valor.
type VariableDeSalida struct {
	Nombre      string
	Descripcion string
	Expresion   string
	// Compartida: lo que produce es del ámbito compartido. Si no, del ámbito del ambiente en ejecución.
	Compartida bool
}

// Asercion es una expresión regular que la salida de un comando tiene que cumplir para que el comando se dé
// por bueno. No produce ninguna variable.
type Asercion struct {
	Descripcion string
	Expresion   string
}

// VariableDeclarada es un literal escrito en el pipeline, que puede usar otras variables. Pertenece a un
// ámbito, no a un paso.
type VariableDeclarada struct {
	Nombre      string
	Descripcion string
	// Ambito es el value del ambiente al que pertenece, y está vacío si es el ámbito compartido.
	Ambito string
	Valor  string
}

// Fichero es un fichero o un enlace del material de un paso.
type Fichero struct {
	// Ruta es relativa al directorio del paso, con '/'.
	Ruta       string
	Contenido  string
	Ejecutable bool
	// Enlace es el destino si es un enlace.
	Enlace string
	// Plantilla dice si algún comando lo interpola. Si no, se copia tal cual.
	Plantilla bool
}

// VariableEstandar es una variable que el pipeline usa sin declararla: el motor garantiza que está siempre.
type VariableEstandar struct {
	Nombre string
	// Metadato: la da quien invoca. Si no, la crea el motor.
	Metadato bool
	// DelPaso: cada paso tiene la suya. Si no, es compartida.
	DelPaso bool
}
