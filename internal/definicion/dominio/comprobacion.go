package dominio

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

// VersionDelFormato es la única schema_version que Comprobar acepta en config.yaml.
const VersionDelFormato = "1"

// Comprobar valida un pipeline declarado y devuelve su forma comprobada, o un error con todos los fallos
// encontrados. El orden en que corren los validadores es crítico — ver doc.go, "Orden de validación", que es la
// referencia única: no se repite aquí para que no queden dos listas que mantener sincronizadas.
func Comprobar(pipelineDeclarado PipelineDeclarado) (*PipelineComprobado, error) {
	mapaIlegibles := make(map[string]string, len(pipelineDeclarado.Ilegibles))
	for _, i := range pipelineDeclarado.Ilegibles {
		mapaIlegibles[i.Fichero] = i.Motivo
	}
	comp := &comprobacion{pipelineDeclarado: pipelineDeclarado, mapaIlegibles: mapaIlegibles}

	if err := (&ValidadorVersion{}).Validar(comp); err != nil {
		return nil, err
	}
	for _, validador := range validadoresDelPipeline {
		validador.Validar(comp)
	}
	if len(comp.fallos) > 0 {
		return nil, comp.resultado()
	}

	variables := make([]VariableDePipelineComprobada, len(comp.variablesDePipeline))
	for i, variable := range comp.variablesDePipeline {
		variables[i] = variable.VariableDePipelineComprobada
	}
	return &PipelineComprobado{
		version:   *pipelineDeclarado.Configuracion.Datos.Version,
		commit:    pipelineDeclarado.Commit,
		hash:      pipelineDeclarado.Hash,
		ambientes: comp.ambientesComprobados,
		pasos:     comp.pasosComprobados,
		variables: variables,
	}, nil
}

// resultadoComprobacion es lo que cada Validador produce: sus fallos, y lo que deriva del
// pipeline declarado para que el siguiente validador lo use. Un validador nuevo que necesite
// dejar algo para los que vienen después agrega el campo aquí, no en comprobacion.
type resultadoComprobacion struct {
	fallos               []Fallo
	ambientesComprobados []AmbienteComprobado
	pasosComprobados     []PasoComprobado
	variablesDePipeline  []variableDePipelineEnComprobacion
	variablesDeComandos  map[string]variableDeSalidaEnComprobacion
}

type comprobacion struct {
	pipelineDeclarado PipelineDeclarado
	mapaIlegibles     map[string]string
	resultadoComprobacion
}

// falla registra un fallo: f identifica dónde ocurre (Invariante, Fichero y, si aplica, Paso/Ambiente); el
// detalle se compone aquí, formateado. f llega como struct con nombres de campo para que Paso y Ambiente —los
// dos posicionales que eran strings intercambiables— no puedan invertirse por error.
func (comp *comprobacion) falla(f Fallo, formato string, args ...any) {
	f.Detalle = fmt.Sprintf(formato, args...)
	comp.fallos = append(comp.fallos, f)
}

func (comp *comprobacion) resultado() error {
	return &FallosDeComprobacion{Fallos: comp.fallos}
}

// ilegible dice si un fichero ya falló al leerse, para no sumarle fallos que solo son consecuencia de eso.
func (comp *comprobacion) ilegible(fichero string) bool {
	_, es := comp.mapaIlegibles[fichero]
	return es
}

var reglasDelFormato = map[string]Regla{
	"code":         ReglaCodigo,
	"instructions": ReglaInstrucciones,
	"variables":    ReglaVariables,
}

// configuracion aplica la entrada de un paso bajo steps, en config.yaml (RD-04 §9.20): parte de los valores por
// defecto y, si el paso tiene entrada propia, la reemplaza campo a campo — cada uno con su propia regla de qué
// significa "no escrito" (aplicarReglas, aplicarEdadMaxima, aplicarAmbito).
func (comp *comprobacion) configuracion(paso *PasoComprobado, escrita *ConfiguracionDePasoDeclarada) {
	paso.Reglas = []Regla{ReglaCodigo, ReglaInstrucciones, ReglaVariables}
	if escrita == nil {
		return
	}
	comp.aplicarReglas(paso, escrita)
	comp.aplicarEdadMaxima(paso, escrita)
	comp.aplicarAmbito(paso, escrita)
}

// aplicarReglas reemplaza las reglas por defecto (las tres) por las escritas: sin rules, el paso las mira todas
// (IT-12 DEC-12.7); con rules: [] las vacía a propósito — lo decide ReglasEscritas, no la longitud de la lista.
func (comp *comprobacion) aplicarReglas(paso *PasoComprobado, escrita *ConfiguracionDePasoDeclarada) {
	if !escrita.ReglasEscritas {
		return
	}
	paso.Reglas = []Regla{}
	for _, token := range escrita.Reglas {
		regla, ok := reglasDelFormato[token]
		switch {
		case !ok:
			comp.falla(Fallo{Invariante: Formato, Fichero: FileConfig, Paso: paso.Nombre}, "la regla %q no existe: code, instructions o variables", token)
		case slices.Contains(paso.Reglas, regla):
			comp.falla(Fallo{Invariante: Formato, Fichero: FileConfig, Paso: paso.Nombre}, "la regla %q está dos veces", token)
		default:
			paso.Reglas = append(paso.Reglas, regla)
		}
	}
}

// aplicarEdadMaxima fija cuánto dura válido un registro de este paso; sin max_age, no caduca (IT-12 DEC-12.7).
func (comp *comprobacion) aplicarEdadMaxima(paso *PasoComprobado, escrita *ConfiguracionDePasoDeclarada) {
	if escrita.EdadMaxima == "" {
		return
	}
	edad, err := time.ParseDuration(escrita.EdadMaxima)
	if err != nil || edad <= 0 {
		comp.falla(Fallo{Invariante: Formato, Fichero: FileConfig, Paso: paso.Nombre}, "max_age %q no es una duración positiva, como 720h",
			escrita.EdadMaxima)
		return
	}
	paso.EdadMaxima = edad
}

// aplicarAmbito declara el ámbito propio del paso (RD-04 §9.19): decide qué ve (las variables declaradas y de
// salida visibles desde ese ámbito, por Ambito.Ve y variableDeSalidaEnComprobacion.laVe), bajo qué ámbito
// archiva su historia, y qué hereda por defecto lo que produce sin scope propio. Sin scope, su ámbito es el que
// representa al ambiente en que se ejecuta — no un tercer concepto: es el mismo Ambito que tendría una variable
// declarada en ese ambiente, solo que asignado por defecto y no por escrito.
func (comp *comprobacion) aplicarAmbito(paso *PasoComprobado, escrita *ConfiguracionDePasoDeclarada) {
	switch escrita.Ambito {
	case "", "environment":
	case "shared":
		paso.Ambito = Compartido.puntero()
	default:
		comp.falla(Fallo{Invariante: Formato, Fichero: FileConfig, Paso: paso.Nombre}, "el paso tiene scope %q, que no existe: es environment o shared",
			escrita.Ambito)
	}
}

// material toma el directorio del paso. Un enlace no puede salir de él: lo que está fuera no es material del
// paso, y su hash no lo vería.
func (comp *comprobacion) material(paso *PasoComprobado, escritos []FicheroDeclarado) {
	for _, f := range escritos {
		if f.Enlace != "" {
			if _, ok := rutaLocal(path.Join(path.Dir(f.Ruta), f.Enlace)); !ok || path.IsAbs(f.Enlace) {
				comp.falla(Fallo{Invariante: Formato, Fichero: paso.Directorio() + "/" + f.Ruta, Paso: paso.Nombre},
					"el enlace apunta a %q, fuera del directorio del paso", f.Enlace)
				continue
			}
		}
		paso.Material = append(paso.Material, FicheroComprobado{
			Ruta: f.Ruta, Contenido: f.Contenido, Ejecutable: f.Ejecutable, Enlace: f.Enlace,
		})
	}
}

func (comp *comprobacion) comandos(paso *PasoComprobado, escritos Declarado[[]ComandoDeclarado]) {
	fichero := paso.Directorio() + "/commands.yaml"
	if !escritos.Existe {
		comp.falla(Fallo{Invariante: Pasos, Fichero: fichero, Paso: paso.Nombre}, "no está, y el paso declara ahí sus comandos")
		return
	}
	if len(escritos.Datos) == 0 && !comp.ilegible(fichero) {
		comp.falla(Fallo{Invariante: Pasos, Fichero: fichero, Paso: paso.Nombre}, "no declara ningún comando")
		return
	}
	procesador := &procesadorComandos{contextoDePaso{comprobacion: comp, paso: paso, fichero: fichero}}
	procesador.procesar(escritos.Datos)
}

// contextoDePaso es lo que procesadorComandos y procesadorOutputs comparten para reportar un fallo: el
// *comprobacion donde se acumula, y el paso y fichero donde ocurre. Antes cada llamada armaba su propio
// Fallo{Fichero: ..., Paso: ...}; ahora es un solo lugar que editar si esa asociación cambia.
type contextoDePaso struct {
	comprobacion *comprobacion
	paso         *PasoComprobado
	fichero      string
}

func (ctx *contextoDePaso) falla(inv Invariante, formato string, args ...any) {
	ctx.comprobacion.falla(Fallo{Invariante: inv, Fichero: ctx.fichero, Paso: ctx.paso.Nombre}, formato, args...)
}

type procesadorComandos struct {
	contextoDePaso
}

func (pc *procesadorComandos) procesar(escritos []ComandoDeclarado) {
	for i, escrito := range escritos {
		pc.procesarUnComando(i, escrito)
	}
}

func (pc *procesadorComandos) procesarUnComando(idx int, escrito ComandoDeclarado) {
	donde := fmt.Sprintf("el comando %d", idx+1)
	comando := ComandoComprobado{
		Nombre: escrito.Nombre, Descripcion: escrito.Descripcion, Linea: escrito.Linea,
	}

	pc.validarLinea(escrito.Linea, donde)
	pc.procesarWorkdir(&comando, escrito.Directorio, donde)
	pc.procesarPlantillas(&comando, escrito.Plantillas, donde)
	pc.comprobacion.outputs(pc.paso, &comando, pc.fichero, donde, escrito.Variables)
	pc.paso.Comandos = append(pc.paso.Comandos, comando)
}

func (pc *procesadorComandos) validarLinea(linea, donde string) {
	if strings.TrimSpace(linea) == "" {
		pc.falla(Formato, "%s no dice cmd", donde)
	}
}

func (pc *procesadorComandos) procesarWorkdir(comando *ComandoComprobado, workdir, donde string) {
	if workdir == "" {
		return
	}
	limpio, ok := rutaLocal(workdir)
	if !ok {
		pc.falla(Formato, "%s tiene workdir %q, que sale del directorio del paso", donde, workdir)
		return
	}
	comando.Directorio = limpio
}

func (pc *procesadorComandos) procesarPlantillas(comando *ComandoComprobado, plantillas []string, donde string) {
	for _, plantilla := range plantillas {
		pc.procesarUnaPlantilla(comando, plantilla, donde)
	}
}

func (pc *procesadorComandos) procesarUnaPlantilla(comando *ComandoComprobado, plantilla, donde string) {
	ruta, ok := rutaLocal(path.Join(comando.Directorio, plantilla))
	if !ok {
		pc.falla(Formato, "%s tiene la plantilla %q, que sale del directorio del paso", donde, plantilla)
		return
	}

	j := pc.buscarMaterial(ruta)
	if j < 0 {
		pc.falla(Formato, "%s tiene la plantilla %q, y %s no está en el material del paso", donde, plantilla, ruta)
		return
	}

	if pc.paso.Material[j].Enlace != "" {
		pc.falla(Formato, "%s tiene la plantilla %q, y %s es un enlace", donde, plantilla, ruta)
		return
	}

	pc.paso.Material[j].Plantilla = true
	if !slices.Contains(comando.Plantillas, ruta) {
		comando.Plantillas = append(comando.Plantillas, ruta)
	}
}

func (pc *procesadorComandos) buscarMaterial(ruta string) int {
	return slices.IndexFunc(pc.paso.Material, func(f FicheroComprobado) bool { return f.Ruta == ruta })
}

// outputs parte cada entrada de outputs en lo que es: con name, una variable de salida, que pertenece a un
// ámbito; sin name y con probe, una aserción, que no produce nada y por eso no tiene ámbito. Sin ninguno de los
// dos no declara nada (RD-04 §9). Sin scope propio, una variable de salida hereda el ámbito del paso que la
// produce (RD-04 §9.19): el compartido si el paso es de scope shared, o si no el del ambiente en que se
// ejecuta.
func (comp *comprobacion) outputs(paso *PasoComprobado, comando *ComandoComprobado, fichero, donde string, escritas []VariableDeComandoDeclarada) {
	procesador := &procesadorOutputs{
		contextoDePaso: contextoDePaso{comprobacion: comp, paso: paso, fichero: fichero},
		comando:        comando,
		donde:          donde,
		nombresVisto:   []string{},
	}
	procesador.procesar(escritas)
}

type procesadorOutputs struct {
	contextoDePaso
	comando      *ComandoComprobado
	donde        string
	nombresVisto []string
}

func (po *procesadorOutputs) procesar(escritas []VariableDeComandoDeclarada) {
	for _, salida := range escritas {
		po.procesarUnOutput(salida)
	}
}

func (po *procesadorOutputs) procesarUnOutput(salida VariableDeComandoDeclarada) {
	if !po.esValido(salida) {
		return
	}

	if salida.Nombre == "" {
		po.procesarAsersion(salida)
		return
	}

	po.procesarVariable(salida)
}

func (po *procesadorOutputs) esValido(salida VariableDeComandoDeclarada) bool {
	if salida.Expresion == "" && salida.Nombre == "" {
		po.falla(Formato, "%s tiene un outputs sin name ni probe: con name es una variable de salida, y sin "+
			"name es una aserción sobre la salida del comando", po.donde)
		return false
	}

	if salida.Expresion != "" {
		if _, err := regexp.Compile(salida.Expresion); err != nil {
			invariante, que := Aserciones, "de una aserción"
			if salida.Nombre != "" {
				invariante, que = VariablesDeSalida, fmt.Sprintf("de %q", salida.Nombre)
			}
			po.falla(invariante, "%s: la expresión regular %s no es correcta: %v", po.donde, que, err)
		}
	}

	return true
}

func (po *procesadorOutputs) procesarAsersion(salida VariableDeComandoDeclarada) {
	if salida.Ambito != "" {
		po.falla(Formato, "%s tiene una aserción con scope %q, y una aserción no produce ninguna variable: el "+
			"ámbito es de lo que se produce", po.donde, salida.Ambito)
	}
	po.comando.Aserciones = append(po.comando.Aserciones, AsercionComprobada{Descripcion: salida.Descripcion, Expresion: salida.Expresion})
}

func (po *procesadorOutputs) procesarVariable(declarada VariableDeComandoDeclarada) {
	if !po.validarNombreVariable(declarada.Nombre) {
		return
	}

	po.nombresVisto = append(po.nombresVisto, declarada.Nombre)
	salida := VariableDeComandoComprobada{Nombre: declarada.Nombre, Descripcion: declarada.Descripcion, Expresion: declarada.Expresion}
	po.asignarAmbito(&salida, declarada.Ambito)
	po.validarProbe(declarada.Nombre, declarada.Expresion)
	po.comando.VariablesDeSalida = append(po.comando.VariablesDeSalida, salida)
}

func (po *procesadorOutputs) validarNombreVariable(nombre string) bool {
	switch {
	case !patronVariable.MatchString(nombre):
		po.falla(Formato, "%s tiene un outputs con name %q, que no es un nombre de variable", po.donde, nombre)
		return false
	case esEstandar(nombre):
		po.falla(Variables, "%s produce %q, que es una variable estándar", po.donde, nombre)
		return false
	case slices.Contains(po.nombresVisto, nombre):
		po.falla(VariablesDeSalida, "%s produce %q dos veces", po.donde, nombre)
		return false
	}
	return true
}

func (po *procesadorOutputs) asignarAmbito(salida *VariableDeComandoComprobada, ambito string) {
	switch ambito {
	case "":
		salida.Ambito = clonarAmbito(po.paso.Ambito)
	case "environment":
	case "shared":
		salida.Ambito = Compartido.puntero()
	default:
		po.falla(Formato, "%s produce %q con scope %q, que no existe: es environment o shared", po.donde, salida.Nombre, ambito)
	}
}

func (po *procesadorOutputs) validarProbe(nombre, expresion string) {
	if expresion == "" {
		po.falla(VariablesDeSalida, "la variable de salida %q no dice probe: es la expresión regular con la que se saca su valor", nombre)
	}
}
