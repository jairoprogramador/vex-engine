package dominio

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

const VersionDelFormato = "1"

// Comprobar valida un pipeline declarado y devuelve su forma comprobada, o un error con todos los fallos encontrados.
// El orden de validación es crítico: cada validador asume que los anteriores no fallaron.
// 1. ValidadorVersion → verifica schema_version (si falla, no se comprueba nada más: todo lo demás fallaría por
// la misma causa)
// 2. ValidadorArchivosIlegibles → reporta I/O y errores de estructura
// 3. ValidadorAmbientes → valida declaración y orden de ambientes
// 4. ValidadorPasos → valida nombres, órdenes y referencias en config.yaml
// 5. ValidadorSalidas → indexa variables de salida de comandos, detecta duplicados
// 6. ValidadorVariablesDeclaradas → valida variables/; no puede ejecutarse antes de ValidadorAmbientes
// 7. ValidadorUsos → valida que las variables usadas existan y sean visibles; no puede ejecutarse antes de
// ValidadorSalidas
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

type comprobacion struct {
	pipelineDeclarado    PipelineDeclarado
	mapaIlegibles        map[string]string
	fallos               []Fallo
	ambientesComprobados []AmbienteComprobado
	pasosComprobados     []PasoComprobado
	variablesDePipeline  []variableDePipelineEnComprobacion
	variablesDeComandos  map[string]variableDeComandoEnComprobacion
}

func (comp *comprobacion) falla(inv Invariante, fichero, paso, ambiente, formato string, args ...any) {
	comp.fallos = append(comp.fallos, Fallo{
		Invariante: inv, Fichero: fichero, Paso: paso, Ambiente: ambiente, Detalle: fmt.Sprintf(formato, args...),
	})
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

// configuracion aplica la entrada de un paso bajo steps, en config.yaml (RD-04 §9.20), con sus valores por
// defecto: sin rules, el paso mira las tres cosas, y sin max_age, no caduca (IT-12 DEC-12.7). scope declara
// el ámbito propio del paso (RD-04 §9.19): decide qué ve (las variables variablesDePipeline y de salida visibles desde
// ese ámbito, por Ambito.Ve y variableDeComandoEnComprobacion.laVe), bajo qué ámbito archiva su historia, y qué hereda por
// defecto lo que produce sin scope propio. Sin scope, su ámbito es el que representa al ambiente en que se
// ejecuta — no un tercer concepto: es el mismo Ambito que tendría una variable variableDePipelineEnComprobacion en ese ambiente,
// solo que asignado por defecto y no por escrito.
func (comp *comprobacion) configuracion(paso *PasoComprobado, escrita *ConfiguracionDePasoDeclarada) {
	paso.Reglas = []Regla{ReglaCodigo, ReglaInstrucciones, ReglaVariables}
	if escrita == nil {
		return
	}
	if escrita.ReglasEscritas {
		paso.Reglas = []Regla{}
		for _, token := range escrita.Reglas {
			regla, ok := reglasDelFormato[token]
			switch {
			case !ok:
				comp.falla(Formato, FileConfig, paso.Nombre, "", "la regla %q no existe: code, instructions o variables", token)
			case slices.Contains(paso.Reglas, regla):
				comp.falla(Formato, FileConfig, paso.Nombre, "", "la regla %q está dos veces", token)
			default:
				paso.Reglas = append(paso.Reglas, regla)
			}
		}
	}
	if escrita.EdadMaxima != "" {
		edad, err := time.ParseDuration(escrita.EdadMaxima)
		if err != nil || edad <= 0 {
			comp.falla(Formato, FileConfig, paso.Nombre, "", "max_age %q no es una duración positiva, como 720h",
				escrita.EdadMaxima)
		}
		paso.EdadMaxima = edad
	}
	switch escrita.Ambito {
	case "", "environment":
	case "shared":
		paso.Ambito = Compartido.puntero()
	default:
		comp.falla(Formato, FileConfig, paso.Nombre, "", "el paso tiene scope %q, que no existe: es environment o shared",
			escrita.Ambito)
	}
}

// material toma el directorio del paso. Un enlace no puede salir de él: lo que está fuera no es material del
// paso, y su hash no lo vería.
func (comp *comprobacion) material(paso *PasoComprobado, escritos []FicheroDeclarado) {
	for _, f := range escritos {
		if f.Enlace != "" {
			if _, ok := rutaLocal(path.Join(path.Dir(f.Ruta), f.Enlace)); !ok || path.IsAbs(f.Enlace) {
				comp.falla(Formato, paso.Directorio()+"/"+f.Ruta, paso.Nombre, "",
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
		comp.falla(Pasos, fichero, paso.Nombre, "", "no está, y el paso declara ahí sus comandos")
		return
	}
	if len(escritos.Datos) == 0 && !comp.ilegible(fichero) {
		comp.falla(Pasos, fichero, paso.Nombre, "", "no declara ningún comando")
		return
	}
	procesador := &procesadorComandos{
		comprobacion: comp,
		paso:         paso,
		fichero:      fichero,
	}
	procesador.procesar(escritos.Datos)
}

type procesadorComandos struct {
	comprobacion *comprobacion
	paso         *PasoComprobado
	fichero      string
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

	if !pc.validarLinea(escrito.Linea, donde) {
		comando.Linea = escrito.Linea
	}

	pc.procesarWorkdir(&comando, escrito.Directorio, donde)
	pc.procesarPlantillas(&comando, escrito.Plantillas, donde)
	pc.comprobacion.outputs(pc.paso, &comando, pc.fichero, donde, escrito.Variables)
	pc.paso.Comandos = append(pc.paso.Comandos, comando)
}

func (pc *procesadorComandos) validarLinea(linea, donde string) bool {
	if strings.TrimSpace(linea) == "" {
		pc.comprobacion.falla(Formato, pc.fichero, pc.paso.Nombre, "", "%s no dice cmd", donde)
		return false
	}
	return true
}

func (pc *procesadorComandos) procesarWorkdir(comando *ComandoComprobado, workdir, donde string) {
	if workdir == "" {
		return
	}
	limpio, ok := rutaLocal(workdir)
	if !ok {
		pc.comprobacion.falla(Formato, pc.fichero, pc.paso.Nombre, "", "%s tiene workdir %q, que sale del directorio del paso",
			donde, workdir)
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
		pc.comprobacion.falla(Formato, pc.fichero, pc.paso.Nombre, "", "%s tiene la plantilla %q, que sale del directorio del paso",
			donde, plantilla)
		return
	}

	j := pc.buscarMaterial(ruta)
	if j < 0 {
		pc.comprobacion.falla(Formato, pc.fichero, pc.paso.Nombre, "", "%s tiene la plantilla %q, y %s no está en el material del paso",
			donde, plantilla, ruta)
		return
	}

	if pc.paso.Material[j].Enlace != "" {
		pc.comprobacion.falla(Formato, pc.fichero, pc.paso.Nombre, "", "%s tiene la plantilla %q, y %s es un enlace",
			donde, plantilla, ruta)
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
		comprobacion: comp,
		paso:         paso,
		comando:      comando,
		fichero:      fichero,
		donde:        donde,
		nombresVisto: []string{},
	}
	procesador.procesar(escritas)
}

type procesadorOutputs struct {
	comprobacion *comprobacion
	paso         *PasoComprobado
	comando      *ComandoComprobado
	fichero      string
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
		po.comprobacion.falla(Formato, po.fichero, po.paso.Nombre, "", "%s tiene un outputs sin name ni probe: con name es una "+
			"variable de salida, y sin name es una aserción sobre la salida del comando", po.donde)
		return false
	}

	if salida.Expresion != "" {
		if _, err := regexp.Compile(salida.Expresion); err != nil {
			invariante, que := Aserciones, "de una aserción"
			if salida.Nombre != "" {
				invariante, que = VariablesDeSalida, fmt.Sprintf("de %q", salida.Nombre)
			}
			po.comprobacion.falla(invariante, po.fichero, po.paso.Nombre, "", "%s: la expresión regular %s no es correcta: %v",
				po.donde, que, err)
		}
	}

	return true
}

func (po *procesadorOutputs) procesarAsersion(salida VariableDeComandoDeclarada) {
	if salida.Ambito != "" {
		po.comprobacion.falla(Formato, po.fichero, po.paso.Nombre, "", "%s tiene una aserción con scope %q, y una aserción no "+
			"produce ninguna variable: el ámbito es de lo que se produce", po.donde, salida.Ambito)
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
		po.comprobacion.falla(Formato, po.fichero, po.paso.Nombre, "", "%s tiene un outputs con name %q, que no es un nombre de "+
			"variable", po.donde, nombre)
		return false
	case esEstandar(nombre):
		po.comprobacion.falla(Variables, po.fichero, po.paso.Nombre, "", "%s produce %q, que es una variable estándar", po.donde, nombre)
		return false
	case slices.Contains(po.nombresVisto, nombre):
		po.comprobacion.falla(VariablesDeSalida, po.fichero, po.paso.Nombre, "", "%s produce %q dos veces", po.donde, nombre)
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
		po.comprobacion.falla(Formato, po.fichero, po.paso.Nombre, "", "%s produce %q con scope %q, que no existe: es environment "+
			"o shared", po.donde, salida.Nombre, ambito)
	}
}

func (po *procesadorOutputs) validarProbe(nombre, expresion string) {
	if expresion == "" {
		po.comprobacion.falla(VariablesDeSalida, po.fichero, po.paso.Nombre, "", "la variable de salida %q no dice probe: es la "+
			"expresión regular con la que se saca su valor", nombre)
	}
}
