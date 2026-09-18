package publicado

// PeticionDeSimulacion es lo que hace falta para simular un pipeline (SIM-1): de una copia de trabajo o de un
// commit (IT-10 DEC-10.6). CopiaDeTrabajo, si viene, tiene prioridad sobre Fuente y Commit — mismo criterio
// que ejecucion/publicado.PeticionDeIntento. Sin Metadatos: nada en Resolución de Variables tiene dónde
// recibirlos para una petición sin intento (RD-09 §9).
type PeticionDeSimulacion struct {
	Version        string // DEC-05.6
	Fuente         string
	Commit         string
	CopiaDeTrabajo string
}

// Informe es el resultado de una simulación (simulacion.md, «Value objects»): nunca lleva valores, solo
// nombres — de qué se resolvió y qué no.
type Informe struct {
	Comprobacion ResultadoDeComprobacion
	// Ambientes está vacío si la comprobación no pasó: sin pipeline comprobado no hay nada que recorrer.
	Ambientes []InformeDeAmbiente
}

// ResultadoDeComprobacion es el primer paso de SIM-1: si el pipeline no pasa la comprobación, ese es el
// resultado entero.
type ResultadoDeComprobacion struct {
	Paso   bool
	Fallos []Fallo
}

// Fallo es un fallo de la comprobación del pipeline, en el lenguaje propio de Simulación (definicion/publicado
// no se puede importar aquí: publicado/ solo usa la biblioteca estándar).
type Fallo struct {
	Invariante string
	Fichero    string
	Paso       string
	Ambiente   string
	Detalle    string
}

// InformeDeAmbiente es lo que se recorrió de un ambiente: sus pasos, en el orden en que se recorrieron. Si un
// paso de este ambiente tuvo una variable que no se pudo interpolar (SIM-2), es el último de la lista.
type InformeDeAmbiente struct {
	Ambiente string
	Pasos    []InformeDePaso
}

// InformeDePaso es lo que pasó en un paso, por nombre — nunca por valor.
type InformeDePaso struct {
	Paso string
	// Interpolado son los nombres de las variables de salida para las que se fabricó una salida simulada en
	// este paso.
	Interpolado []string
	// Faltante son los nombres que no se pudieron interpolar en este paso (SIM-2). Si no está vacío, el
	// ambiente se abandonó después de este paso.
	Faltante []string
}
