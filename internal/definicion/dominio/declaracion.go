package dominio

type Declarado[T any] struct {
	Existe bool
	Datos  T
}

type PipelineDeclarado struct {
	Commit        string
	Hash          string
	Configuracion Declarado[ConfiguracionDeclarada]
	Ambientes     Declarado[[]AmbienteDeclarado]
	Pasos         []PasoDeclarado
	Variables     []VariablesDePipelineDeclarada
	Ilegibles     []FicheroIlegibleDeclarado
	Desconocidos  []string
}

type ConfiguracionDeclarada struct {
	Version *string
	Pasos   map[string]ConfiguracionDePasoDeclarada
}

type FicheroIlegibleDeclarado struct {
	Fichero string
	Motivo  string
}

type AmbienteDeclarado struct {
	Nombre      string
	Descripcion string
	Valor       string
}

type PasoDeclarado struct {
	Directorio string
	Comandos   Declarado[[]ComandoDeclarado]
	Material   []FicheroDeclarado
}

type ComandoDeclarado struct {
	Nombre      string
	Descripcion string
	Linea       string
	Directorio  string
	Plantillas  []string
	Variables   []VariableDeComandoDeclarada
}

type VariableDeComandoDeclarada struct {
	Nombre      string
	Descripcion string
	Expresion   string
	Ambito      string
}

type VariableDePipelineDeclarada struct {
	Nombre      string
	Descripcion string
	Valor       *string
}

type VariablesDePipelineDeclarada struct {
	Fichero   string
	Ambito    string
	Variables []VariableDePipelineDeclarada
}

type ConfiguracionDePasoDeclarada struct {
	Reglas         []string
	ReglasEscritas bool
	EdadMaxima     string
	Ambito         string
}

type FicheroDeclarado struct {
	Ruta       string
	Contenido  string
	Ejecutable bool
	Enlace     string
}
