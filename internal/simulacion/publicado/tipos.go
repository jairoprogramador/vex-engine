package publicado

// PeticionDeSimulacion es lo que hace falta para simular un pipeline (SIM-1), con la misma forma que
// ejecucion/publicado.PeticionDeIntento: hasta qué paso y en qué ambiente se ejecutaría, y quién lo pide. El
// pipeline sale de una copia de trabajo o de un commit (IT-10 DEC-10.6); CopiaDeTrabajo, si viene, tiene
// prioridad sobre Fuente y Commit.
type PeticionDeSimulacion struct {
	Version        string // DEC-05.6
	Ambiente       string // el valor del ambiente (environments.yaml), no su nombre
	Solicitante    string
	HastaPaso      string
	Fuente         string
	Commit         string
	CopiaDeTrabajo string
	Metadatos      Metadatos
}

// Metadatos son los datos de la variable estándar que solo puede dar quien invoca, los mismos que da a un
// intento (ejecucion/publicado.Metadatos): sin ellos, project_name y las demás quedan vacías, igual que en un
// intento sin metadatos.
type Metadatos struct {
	ProjectId           string
	ProjectName         string
	ProjectOrganization string
	ProjectTeam         string
}

// Estado es cómo habría terminado el intento: el mismo vocabulario que el resumen de un intento.
type Estado string

const (
	EstadoExitoso Estado = "exitoso"
	EstadoFallido Estado = "fallido"
)

// Resultado es el resumen de una simulación, como el resumen de un intento pero sin Id: no se guarda nada.
// Causa solo viene si el Estado es fallido, y nunca lleva valores, solo nombres.
type Resultado struct {
	Ambiente    string
	Solicitante string
	HastaPaso   string
	Estado      Estado
	Causa       *Causa `json:",omitempty"`
}

// Causa dice por qué un intento con estos datos fallaría: o el pipeline no pasa la comprobación (Fallos), o
// hay variables que no se pueden interpolar (Faltante). Nunca las dos a la vez: sin pipeline comprobado no
// hay nada que interpolar.
type Causa struct {
	Fallos   []Fallo          `json:",omitempty"`
	Faltante []FaltanteDePaso `json:",omitempty"`
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

// FaltanteDePaso son los nombres que no se pudieron interpolar en un paso (SIM-2). Con el primer paso que
// tenga alguno se deja de simular: es el único de la lista.
type FaltanteDePaso struct {
	Paso      string
	Variables []string
}
