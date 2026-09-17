package dominio

import (
	"io/fs"
	"path"
	"regexp"
	"slices"
)

type Ambito string
type Regla string
type OrigenEstandar int

const Compartido Ambito = ""

// Rutas y ficheros del pipeline
const (
	DirVariables     = "variables/"
	DirSteps         = "steps/"
	DirCommands      = "commands/"
	FileConfig       = "config.yaml"
	FileEnvironments = "environments.yaml"
	FileCommands     = "commands.yaml"
	FileVExPipeline  = "vexpipeline.yaml"
	PatternStepsDir  = "NN-<step>"
)

const (
	ReglaCodigo        Regla = "codigo"
	ReglaInstrucciones Regla = "instrucciones"
	ReglaVariables     Regla = "variables"
)

const (
	Metadato OrigenEstandar = iota
	Generada
)

type VariableEstandar struct {
	Nombre  string
	Origen  OrigenEstandar
	DelPaso bool
}

var variablesEstandar = []VariableEstandar{
	{Nombre: "project_id", Origen: Metadato},
	{Nombre: "project_name", Origen: Metadato},
	{Nombre: "project_organization", Origen: Metadato},
	{Nombre: "project_team", Origen: Metadato},
	{Nombre: "environment", Origen: Metadato},
	{Nombre: "project_hash", Origen: Generada},
	{Nombre: "project_version", Origen: Generada},
	{Nombre: "project_workdir", Origen: Generada},
	{Nombre: "tool_name", Origen: Generada},
	{Nombre: "step_name", Origen: Generada, DelPaso: true},
	{Nombre: "step_workdir", Origen: Generada, DelPaso: true},
}

func VariablesEstandar() []VariableEstandar { return slices.Clone(variablesEstandar) }

func esEstandar(nombre string) bool {
	return slices.ContainsFunc(variablesEstandar, func(v VariableEstandar) bool { return v.Nombre == nombre })
}

var (
	patronDirectorioDePaso = regexp.MustCompile(`^([0-9]{2})-(.*)$`)
	patronNombre           = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	patronVariable         = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	patronUso              = regexp.MustCompile(`\$\{var\.([^}]*)\}`)
)

func usos(texto string) (nombres, malformados []string) {
	for _, m := range patronUso.FindAllStringSubmatch(texto, -1) {
		targetList := &nombres
		if !patronVariable.MatchString(m[1]) {
			targetList = &malformados
		}
		if !slices.Contains(*targetList, m[1]) {
			*targetList = append(*targetList, m[1])
		}
	}
	return nombres, malformados
}

func rutaLocal(ruta string) (string, bool) {
	cleanPath := path.Clean(ruta)
	return cleanPath, fs.ValidPath(cleanPath)
}

func (a Ambito) EsCompartido() bool { return a == Compartido }

func (a Ambito) deUnAmbiente() string {
	if a.EsCompartido() {
		return ""
	}
	return string(a)
}

func (a Ambito) Ve(otro Ambito) bool { return otro == a || otro.EsCompartido() }

func (a Ambito) String() string {
	if a.EsCompartido() {
		return "compartido"
	}
	return string(a)
}

func (a Ambito) puntero() *Ambito { return &a }

func clonarAmbito(a *Ambito) *Ambito {
	if a == nil {
		return nil
	}
	return a.puntero()
}
