package dominio

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

const VersionDelFormato = "1"

// Comprobar valida un pipeline declarado y devuelve su forma comprobada, o un error con todos los fallos encontrados.
// El orden de validación es crítico: cada validador asume que los anteriores no fallaron.
// 1. version() → verifica schema_version (si falla, todo falla)
// 2. Archivos ilegibles y desconocidos → reporta I/O y errores de estructura
// 3. ambientes() → valida declaración y orden de ambientes
// 4. pasos() → valida nombres, órdenes y referencias en config.yaml
// 5. salidas() → indexa variables de salida de comandos, detecta duplicados
// 6. variablesDeclaradas() → valida variables/; no puede ejecutarse antes de ambientes
// 7. usos() → valida que las variables usadas existan y sean visibles; no puede ejecutarse antes de salidas
func Comprobar(pipelineDeclarado PipelineDeclarado) (*PipelineComprobado, error) {
	comprobacion := &comprobacion{pipelineDeclarado: pipelineDeclarado}
	if !comprobacion.version() {
		return nil, comprobacion.resultado()
	}
	for _, i := range pipelineDeclarado.Ilegibles {
		comprobacion.falla(Formato, i.Fichero, "", "", "%s", i.Motivo)
	}
	for _, ruta := range pipelineDeclarado.Desconocidos {
		comprobacion.falla(Formato, ruta, "", "", "no es parte del formato del pipeline")
	}
	comprobacion.ambientes()
	comprobacion.pasos()
	comprobacion.salidas()
	comprobacion.variablesDeclaradas()
	comprobacion.usos()
	if len(comprobacion.fallos) > 0 {
		return nil, comprobacion.resultado()
	}
	variables := make([]VariableDePipelineComprobada, len(comprobacion.variablesDePipeline))
	for i, d := range comprobacion.variablesDePipeline {
		variables[i] = d.VariableDePipelineComprobada
	}
	return &PipelineComprobado{
		version:   *pipelineDeclarado.Configuracion.Datos.Version,
		commit:    pipelineDeclarado.Commit,
		hash:      pipelineDeclarado.Hash,
		ambientes: comprobacion.ambientesComprobados,
		pasos:     comprobacion.pasosComprobados,
		variables: variables,
	}, nil
}

type comprobacion struct {
	pipelineDeclarado    PipelineDeclarado
	fallos               []Fallo
	ambientesComprobados []AmbienteComprobado
	pasosComprobados     []PasoComprobado
	variablesDePipeline  []variableDePipelineEnComprobacion
	variablesDeComandos  map[string]variableDeComandoEnComprobacion
	consumidas           map[string]bool
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
	return slices.ContainsFunc(comp.pipelineDeclarado.Ilegibles, func(i FicheroIlegibleDeclarado) bool { return i.Fichero == fichero })
}

// version rechaza cualquier formato que no sea VersionDelFormato, y entonces no comprueba nada más: todo lo
// demás fallaría por la misma causa.
func (comp *comprobacion) version() bool {
	switch {
	case !comp.pipelineDeclarado.Configuracion.Existe:
		comp.falla(Formato, FileConfig, "", "", "no está, y el pipeline declara ahí su schema_version")
	case comp.ilegible(FileConfig):
		for _, i := range comp.pipelineDeclarado.Ilegibles {
			if i.Fichero == FileConfig {
				comp.falla(Formato, FileConfig, "", "", "%s", i.Motivo)
			}
		}
	case comp.pipelineDeclarado.Configuracion.Datos.Version == nil:
		comp.falla(Formato, FileConfig, "", "", "no dice schema_version, y la única que se lee es la %s", VersionDelFormato)
	case *comp.pipelineDeclarado.Configuracion.Datos.Version != VersionDelFormato:
		comp.falla(Formato, FileConfig, "", "", "schema_version %q no se lee: la única que se lee es la %s",
			*comp.pipelineDeclarado.Configuracion.Datos.Version, VersionDelFormato)
	default:
		return true
	}
	return false
}

func (comp *comprobacion) ambientes() {
	if !comp.pipelineDeclarado.Ambientes.Existe {
		comp.falla(Ambientes, FileEnvironments, "", "", "no está, y el pipeline declara ahí sus ambientes en orden")
		return
	}
	if len(comp.pipelineDeclarado.Ambientes.Datos) == 0 && !comp.ilegible(FileEnvironments) {
		comp.falla(Ambientes, FileEnvironments, "", "", "no declara ningún ambiente")
		return
	}
	validador := &validadorAmbientes{
		comprobacion: comp,
		fichero:      FileEnvironments,
		nombres:      []string{},
		valores:      []string{},
	}
	validador.validar(comp.pipelineDeclarado.Ambientes.Datos)
}

type validadorAmbientes struct {
	comprobacion *comprobacion
	fichero      string
	nombres      []string
	valores      []string
}

func (v *validadorAmbientes) validar(ambientes []AmbienteDeclarado) {
	for i, a := range ambientes {
		v.validarUnAmbiente(i, a)
	}
}

func (v *validadorAmbientes) validarUnAmbiente(idx int, a AmbienteDeclarado) {
	donde := fmt.Sprintf("el ambiente %d", idx+1)
	if !v.esAmbienteValido(a, donde) {
		return
	}
	v.nombres = append(v.nombres, a.Nombre)
	v.valores = append(v.valores, a.Valor)
	v.comprobacion.ambientesComprobados = append(v.comprobacion.ambientesComprobados, AmbienteComprobado(a))
}

func (v *validadorAmbientes) esAmbienteValido(a AmbienteDeclarado, donde string) bool {
	valido := true

	if a.Nombre == "" {
		v.comprobacion.falla(Formato, v.fichero, "", "", "%s no dice name", donde)
		valido = false
	}

	if !patronNombre.MatchString(a.Valor) {
		v.comprobacion.falla(Formato, v.fichero, "", "", "%s tiene value %q, que no sirve como directorio de variables/: "+
			"letras, dígitos, - y _", donde, a.Valor)
		valido = false
	}

	if contieneSinMayusculas(v.nombres, a.Nombre) {
		v.comprobacion.falla(Ambientes, v.fichero, "", "", "el name %q está dos veces", a.Nombre)
		valido = false
	}

	if contieneSinMayusculas(v.valores, a.Valor) {
		v.comprobacion.falla(Ambientes, v.fichero, "", "", "el value %q está dos veces", a.Valor)
		valido = false
	}

	return valido
}

func (comp *comprobacion) pasos() {
	if len(comp.pipelineDeclarado.Pasos) == 0 {
		comp.falla(Pasos, DirSteps, "", "", "el pipeline no tiene pasos")
		return
	}
	validador := &validadorPasos{
		comprobacion: comp,
		porOrden:     make(map[int]string),
		nombres:      []string{},
		consumidas:   make(map[string]bool),
	}
	validador.validar(comp.pipelineDeclarado.Pasos)
	validador.ordenar()
	validador.validarPasosNoUsados(comp.pipelineDeclarado.Configuracion.Datos.Pasos)
}

type validadorPasos struct {
	comprobacion *comprobacion
	porOrden     map[int]string
	nombres      []string
	consumidas   map[string]bool
}

func (v *validadorPasos) validar(pasos []PasoDeclarado) {
	for _, escrito := range pasos {
		v.validarUnPaso(escrito)
	}
}

func (v *validadorPasos) validarUnPaso(escrito PasoDeclarado) {
	directorio := DirSteps + escrito.Directorio
	matches := patronDirectorioDePaso.FindStringSubmatch(escrito.Directorio)
	if matches == nil {
		v.comprobacion.falla(Pasos, directorio, "", "", "el directorio de un paso se llama NN-<paso>, con NN de dos dígitos")
		return
	}

	orden, _ := strconv.Atoi(matches[1])
	nombre := matches[2]

	if !v.esPasoValido(directorio, nombre, orden, matches[1]) {
		return
	}

	v.procesarPaso(escrito, nombre, orden)
}

func (v *validadorPasos) esPasoValido(directorio, nombre string, orden int, ordenStr string) bool {
	if !patronNombre.MatchString(nombre) {
		v.comprobacion.falla(Pasos, directorio, "", "", "%q no sirve como nombre de un paso: letras, dígitos, - y _", nombre)
		return false
	}

	if otro, repetido := v.porOrden[orden]; repetido {
		v.comprobacion.falla(Pasos, directorio, nombre, "", "el orden %s ya es de steps/%s", ordenStr, otro)
		return false
	}

	if contieneSinMayusculas(v.nombres, nombre) {
		v.comprobacion.falla(Pasos, directorio, nombre, "", "otro paso ya se llama %q", nombre)
		return false
	}

	return true
}

func (v *validadorPasos) procesarPaso(escrito PasoDeclarado, nombre string, orden int) {
	v.porOrden[orden] = escrito.Directorio
	v.nombres = append(v.nombres, nombre)
	v.consumidas[nombre] = true

	paso := PasoComprobado{Nombre: nombre, Orden: orden}
	configuracion, tiene := v.comprobacion.pipelineDeclarado.Configuracion.Datos.Pasos[nombre]
	if tiene {
		v.comprobacion.configuracion(&paso, &configuracion)
	} else {
		v.comprobacion.configuracion(&paso, nil)
	}
	v.comprobacion.material(&paso, escrito.Material)
	v.comprobacion.comandos(&paso, escrito.Comandos)
	v.comprobacion.pasosComprobados = append(v.comprobacion.pasosComprobados, paso)
}

func (v *validadorPasos) ordenar() {
	sort.Slice(v.comprobacion.pasosComprobados, func(i, j int) bool {
		return v.comprobacion.pasosComprobados[i].Orden < v.comprobacion.pasosComprobados[j].Orden
	})
}

func (v *validadorPasos) validarPasosNoUsados(pasosCfg map[string]ConfiguracionDePasoDeclarada) {
	var sinPaso []string
	for nombre := range pasosCfg {
		if !v.consumidas[nombre] {
			sinPaso = append(sinPaso, nombre)
		}
	}
	sort.Strings(sinPaso)
	for _, nombre := range sinPaso {
		v.comprobacion.falla(Pasos, FileConfig, "", "", "declara la configuración de %q, que no es un paso: no hay "+
			"ningún steps/NN-%s/", nombre, nombre)
	}
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

func (p *procesadorOutputs) procesar(escritas []VariableDeComandoDeclarada) {
	for _, salida := range escritas {
		p.procesarUnOutput(salida)
	}
}

func (p *procesadorOutputs) procesarUnOutput(salida VariableDeComandoDeclarada) {
	if !p.esValido(salida) {
		return
	}

	if salida.Nombre == "" {
		p.procesarAsersion(salida)
		return
	}

	p.procesarVariable(salida)
}

func (p *procesadorOutputs) esValido(salida VariableDeComandoDeclarada) bool {
	if salida.Expresion == "" && salida.Nombre == "" {
		p.comprobacion.falla(Formato, p.fichero, p.paso.Nombre, "", "%s tiene un outputs sin name ni probe: con name es una "+
			"variable de salida, y sin name es una aserción sobre la salida del comando", p.donde)
		return false
	}

	if salida.Expresion != "" {
		if _, err := regexp.Compile(salida.Expresion); err != nil {
			invariante, que := Aserciones, "de una aserción"
			if salida.Nombre != "" {
				invariante, que = VariablesDeSalida, fmt.Sprintf("de %q", salida.Nombre)
			}
			p.comprobacion.falla(invariante, p.fichero, p.paso.Nombre, "", "%s: la expresión regular %s no es correcta: %v",
				p.donde, que, err)
		}
	}

	return true
}

func (p *procesadorOutputs) procesarAsersion(salida VariableDeComandoDeclarada) {
	if salida.Ambito != "" {
		p.comprobacion.falla(Formato, p.fichero, p.paso.Nombre, "", "%s tiene una aserción con scope %q, y una aserción no "+
			"produce ninguna variable: el ámbito es de lo que se produce", p.donde, salida.Ambito)
	}
	p.comando.Aserciones = append(p.comando.Aserciones, AsercionComprobada{Descripcion: salida.Descripcion, Expresion: salida.Expresion})
}

func (p *procesadorOutputs) procesarVariable(declarada VariableDeComandoDeclarada) {
	if !p.validarNombreVariable(declarada.Nombre) {
		return
	}

	p.nombresVisto = append(p.nombresVisto, declarada.Nombre)
	salida := VariableDeComandoComprobada{Nombre: declarada.Nombre, Descripcion: declarada.Descripcion, Expresion: declarada.Expresion}
	p.asignarAmbito(&salida, declarada.Ambito)
	p.validarProbe(declarada.Nombre, declarada.Expresion)
	p.comando.VariablesDeSalida = append(p.comando.VariablesDeSalida, salida)
}

func (p *procesadorOutputs) validarNombreVariable(nombre string) bool {
	switch {
	case !patronVariable.MatchString(nombre):
		p.comprobacion.falla(Formato, p.fichero, p.paso.Nombre, "", "%s tiene un outputs con name %q, que no es un nombre de "+
			"variable", p.donde, nombre)
		return false
	case esEstandar(nombre):
		p.comprobacion.falla(Variables, p.fichero, p.paso.Nombre, "", "%s produce %q, que es una variable estándar", p.donde, nombre)
		return false
	case slices.Contains(p.nombresVisto, nombre):
		p.comprobacion.falla(VariablesDeSalida, p.fichero, p.paso.Nombre, "", "%s produce %q dos veces", p.donde, nombre)
		return false
	}
	return true
}

func (p *procesadorOutputs) asignarAmbito(salida *VariableDeComandoComprobada, ambito string) {
	switch ambito {
	case "":
		salida.Ambito = clonarAmbito(p.paso.Ambito)
	case "environment":
	case "shared":
		salida.Ambito = Compartido.puntero()
	default:
		p.comprobacion.falla(Formato, p.fichero, p.paso.Nombre, "", "%s produce %q con scope %q, que no existe: es environment "+
			"o shared", p.donde, salida.Nombre, ambito)
	}
}

func (p *procesadorOutputs) validarProbe(nombre, expresion string) {
	if expresion == "" {
		p.comprobacion.falla(VariablesDeSalida, p.fichero, p.paso.Nombre, "", "la variable de salida %q no dice probe: es la "+
			"expresión regular con la que se saca su valor", nombre)
	}
}

// salidas indexa dónde se produce cada variable de salida. Una variable de salida pertenece a un ámbito y no a
// un paso, así que dos comandos del mismo ámbito no pueden producir el mismo nombre: quien lo usara no sabría
// cuál de los dos ve.
func (comp *comprobacion) salidas() {
	comp.variablesDeComandos = map[string]variableDeComandoEnComprobacion{}
	for i, paso := range comp.pasosComprobados {
		comp.salidasDelPaso(i, paso)
	}
}

func (comp *comprobacion) salidasDelPaso(idxPaso int, paso PasoComprobado) {
	for j, comando := range paso.Comandos {
		comp.salidasDelComando(idxPaso, j, paso, comando)
	}
}

func (comp *comprobacion) salidasDelComando(idxPaso, idxCmd int, paso PasoComprobado, comando ComandoComprobado) {
	for _, s := range comando.VariablesDeSalida {
		comp.registrarSalida(idxPaso, idxCmd, paso, s)
	}
}

func (comp *comprobacion) registrarSalida(idxPaso, idxCmd int, paso PasoComprobado, s VariableDeComandoComprobada) {
	anterior, existia := comp.variablesDeComandos[s.Nombre]
	if !existia {
		comp.variablesDeComandos[s.Nombre] = variableDeComandoEnComprobacion{
			nombre: s.Nombre, donde: posicion{paso: idxPaso, comando: idxCmd}, paso: paso.Nombre, compartida: s.Ambito != nil,
		}
		return
	}

	comp.validarDuplicadoDeSalida(idxCmd, paso, s, anterior)
}

func (comp *comprobacion) validarDuplicadoDeSalida(idxCmd int, paso PasoComprobado, s VariableDeComandoComprobada, anterior variableDeComandoEnComprobacion) {
	fichero := paso.Directorio() + "/commands.yaml"

	if anterior.paso == paso.Nombre && anterior.donde.comando == idxCmd {
		return
	}

	if anterior.compartida != (s.Ambito != nil) {
		comp.falla(VariablesDeSalida, fichero, paso.Nombre, "", "el comando %d produce %q, que el comando %d "+
			"de %q produce en el otro ámbito: un nombre pertenece a un solo ámbito", idxCmd+1, s.Nombre,
			anterior.donde.comando+1, anterior.paso)
		return
	}

	comp.falla(VariablesDeSalida, fichero, paso.Nombre, "", "el comando %d produce %q, que ya produce el "+
		"comando %d de %q: en un ámbito, una variable de salida se produce en un solo sitio", idxCmd+1,
		s.Nombre, anterior.donde.comando+1, anterior.paso)
}

// variablesDeclaradas pone cada fichero de variables/ en su ámbito: variables/<ambiente>/ declara las del
// ámbito de ese ambiente, y la raíz de variables/, las del ámbito compartido (IT-12 DEC-12.7). El nombre del
// fichero solo organiza: las variables son del ámbito, y las ve todo paso que se ejecuta en él.
func (comp *comprobacion) variablesDeclaradas() {
	// donde recuerda en qué ámbitos ya está declarado cada nombre, para ver los que chocan.
	donde := map[string][]variableDePipelineEnComprobacion{}
	for _, escritas := range comp.pipelineDeclarado.Variables {
		fichero := escritas.Fichero
		ambito := Compartido
		if escritas.Ambito != "" {
			ambito = Ambito(escritas.Ambito)
		}
		if !ambito.EsCompartido() && !slices.ContainsFunc(comp.ambientesComprobados,
			func(a AmbienteComprobado) bool { return a.Valor == escritas.Ambito }) {
			comp.falla(Variables, fichero, "", "", DirVariables+"%s/ no es de ningún ámbito: un directorio de "+DirVariables+
				"es el value de un ambiente, y las variables compartidas van en la raíz", escritas.Ambito)
			continue
		}
		for _, v := range escritas.Variables {
			if !comp.nombreDeclarable(fichero, ambito, v.Nombre) {
				continue
			}
			if otra, choca := choque(donde[v.Nombre], ambito); choca {
				if otra.Ambito == ambito {
					comp.falla(Variables, fichero, "", ambito.deUnAmbiente(), "la variable %q ya está declarada en %s",
						v.Nombre, otra.fichero)
				} else {
					comp.falla(Variables, fichero, "", "", "la variable %q ya está declarada en %s, que es del ámbito %s, "+
						"y un paso ve los dos: un nombre pertenece a un solo ámbito", v.Nombre, otra.fichero, otra.Ambito)
				}
				continue
			}
			nueva := variableDePipelineEnComprobacion{VariableDePipelineComprobada: comp.variable(fichero, ambito, v), fichero: fichero}
			donde[v.Nombre] = append(donde[v.Nombre], nueva)
			comp.variablesDePipeline = append(comp.variablesDePipeline, nueva)
		}
	}
}

// choque dice si un nombre ya declarado choca con este ámbito. Dos ambientes distintos no chocan, porque no se
// ven a la vez; el ámbito compartido choca con todos, porque se ve desde todos.
// nombreDeclarable dice si un nombre se puede declarar: es un nombre de variable y no es una variable estándar.
func (comp *comprobacion) nombreDeclarable(fichero string, ambito Ambito, nombre string) bool {
	switch {
	case nombre == "":
		comp.falla(Formato, fichero, "", ambito.deUnAmbiente(), "una variable no dice name")
	case !patronVariable.MatchString(nombre):
		comp.falla(Formato, fichero, "", ambito.deUnAmbiente(), "%q no es un nombre de variable", nombre)
	case esEstandar(nombre):
		comp.falla(Variables, fichero, "", ambito.deUnAmbiente(), "%q es una variable estándar: el motor la da siempre, "+
			"y no se declara", nombre)
	default:
		return true
	}
	return false
}

// variable lee el valor escrito. Si no está, la variable queda variableDePipelineEnComprobacion sin valor: lo que la usa no suma un
// «no variableDePipelineEnComprobacion» que no es verdad, y con el fallo no hay pipeline.
func (comp *comprobacion) variable(fichero string, ambito Ambito, v VariableDePipelineDeclarada) VariableDePipelineComprobada {
	declarada := VariableDePipelineComprobada{Nombre: v.Nombre, Descripcion: v.Descripcion, Ambito: ambito}
	if v.Valor == nil {
		comp.falla(Formato, fichero, "", ambito.deUnAmbiente(), "la variable %q no dice value", v.Nombre)
		return declarada
	}
	declarada.Valor = *v.Valor
	return declarada
}

// usos comprueba que toda variable usada en un comando, en una plantilla o en un valor declarado sea una
// variable estándar, esté variableDePipelineEnComprobacion en un ámbito que se vea desde donde se usa, o la produzca un comando; y
// que lo que necesita esté producido antes de usarse (IT-02 DEC-02.7).
//
// Quien se ejecuta en un ambiente ve: las variables estándar; las variablesDePipeline en el ámbito de ese ambiente y en
// el compartido; y las variables de salida producidas antes, tanto por los comandos anteriores del mismo paso
// como por los pasos anteriores. Lo compartido no ve lo del ambiente: un valor compartido que dependiera de un
// ambiente dejaría de ser el mismo en todos.
func (comp *comprobacion) usos() {
	comp.circulos()
	comp.valoresDeclarados()
	comp.usosEnLosPasos()
}

// declaradaVisible busca un nombre entre las variables variablesDePipeline que se ven desde un ámbito.
func (comp *comprobacion) declaradaVisible(nombre string, ambito Ambito) (variableDePipelineEnComprobacion, bool) {
	for _, variable := range comp.variablesDePipeline {
		if variable.Nombre == nombre && ambito.Ve(variable.Ambito) {
			return variable, true
		}
	}
	return variableDePipelineEnComprobacion{}, false
}

// declaradaEsVisible es declaradaVisible cuando solo hace falta saber si está.
func (comp *comprobacion) declaradaEsVisible(nombre string, ambito Ambito) bool {
	_, hay := comp.declaradaVisible(nombre, ambito)
	return hay
}

// ambitos son aquellos en los que se declara algo: el compartido y el de cada ambiente.
func (comp *comprobacion) ambitos() []Ambito {
	ambitos := []Ambito{Compartido}
	for _, a := range comp.ambientesComprobados {
		ambitos = append(ambitos, Ambito(a.Valor))
	}
	return ambitos
}

// valoresDeclarados comprueba lo que usa cada valor declarado, sin mirar el orden: cuándo se resuelve una
// variable variableDePipelineEnComprobacion depende de dónde se use, y eso se mira en usosEnLosPasos.
func (comp *comprobacion) valoresDeclarados() {
	for _, variable := range comp.variablesDePipeline {
		nombres, malformados := usos(variable.Valor)
		ambiente := variable.Ambito.deUnAmbiente()
		for _, nombre := range malformados {
			comp.falla(Formato, variable.fichero, "", ambiente, "${var.%s} no es un nombre de variable", nombre)
		}
		for _, nombre := range nombres {
			switch salida, produce := comp.variablesDeComandos[nombre]; {
			case esEstandar(nombre) || comp.declaradaEsVisible(nombre, variable.Ambito):
			case produce && salida.laVe(variable.Ambito):
			case produce:
				comp.falla(Variables, variable.fichero, "", ambiente, "%q usa ${var.%s}, que es una variable de salida del "+
					"ámbito de un ambiente, y desde el ámbito compartido no se ve", variable.Nombre, nombre)
			default:
				comp.falla(Variables, variable.fichero, "", ambiente, "%q usa ${var.%s}, que no es una variable estándar, ni "+
					"está declarada en un ámbito que se vea desde aquí, ni la produce ningún comando", variable.Nombre, nombre)
			}
		}
	}
}

// usosEnLosPasos recorre los pasos en su orden y, dentro de cada uno, sus comandos: cada uno ve lo que se
// produjo antes que él, desde el ámbito del paso. Se recorre una vez por ambiente, porque el ámbito de un paso
// sin scope propio es el de ese ambiente (RD-04 §9.19), y lo que se ve cambia con él; un fallo que ocurre en
// todos es uno solo, sin ambiente. El de un paso con scope: shared es siempre el compartido, así que su
// resultado no varía entre ambientes y se reporta igual, sin ambiente.
func (comp *comprobacion) usosEnLosPasos() {
	probs := &problemas{}
	for _, a := range comp.ambientesComprobados {
		comp.usosEnUnAmbiente(probs, Ambito(a.Valor))
	}
	probs.reportar(comp, len(comp.ambientesComprobados))
}

func (comp *comprobacion) usosEnUnAmbiente(probs *problemas, ambiente Ambito) {
	for i, paso := range comp.pasosComprobados {
		ambito := comp.ambientoDePaso(ambiente, paso)
		comp.usosEnUnPaso(probs, i, paso, ambito)
	}
}

func (comp *comprobacion) ambientoDePaso(ambiente Ambito, paso PasoComprobado) Ambito {
	if paso.Ambito != nil {
		return *paso.Ambito
	}
	return ambiente
}

func (comp *comprobacion) usosEnUnPaso(probs *problemas, idxPaso int, paso PasoComprobado, ambito Ambito) {
	for j, comando := range paso.Comandos {
		punto := posicion{paso: idxPaso, comando: j}
		comp.revisar(probs, paso, paso.Directorio()+"/commands.yaml", comando.Linea, ambito, punto)
		comp.revisarPlantillas(probs, paso, idxPaso, j, comando, ambito)
	}
}

func (comp *comprobacion) revisarPlantillas(probs *problemas, paso PasoComprobado, idxPaso, idxCmd int, comando ComandoComprobado, ambito Ambito) {
	for _, ruta := range comando.Plantillas {
		k := slices.IndexFunc(paso.Material, func(f FicheroComprobado) bool { return f.Ruta == ruta })
		if k >= 0 {
			punto := posicion{paso: idxPaso, comando: idxCmd}
			comp.revisar(probs, paso, paso.Directorio()+"/"+ruta, paso.Material[k].Contenido, ambito, punto)
		}
	}
}

// revisar mira un texto que se interpola en un punto del pipeline: cada nombre que usa tiene que verse desde
// ese ámbito, y lo que hace falta para resolverlo tiene que estar producido antes.
func (comp *comprobacion) revisar(probs *problemas, paso PasoComprobado, fichero, texto string, ambito Ambito, punto posicion) {
	nombres, malformados := usos(texto)
	for _, nombre := range malformados {
		probs.anotar(Formato, fichero, paso.Nombre, "", fmt.Sprintf("${var.%s} no es un nombre de variable", nombre))
	}
	for _, nombre := range nombres {
		if !comp.seVe(probs, paso, fichero, nombre, ambito) {
			continue
		}
		for _, necesaria := range comp.necesita(nombre, ambito, map[string]bool{}) {
			if necesaria.donde.antesDe(punto) {
				continue
			}
			probs.anotar(Variables, fichero, paso.Nombre, ambito.deUnAmbiente(), tarde(nombre, necesaria, punto))
		}
	}
}

// seVe dice si un nombre usado dentro de un paso se ve desde su ámbito, y si no, lo anota: el orden lo mira
// revisar, no esto. Una variable de salida solo se ve si su ámbito la deja ver desde aquí (variableDeComandoEnComprobacion.laVe):
// un paso de scope: shared ve las suyas, no las de un ambiente.
func (comp *comprobacion) seVe(probs *problemas, paso PasoComprobado, fichero, nombre string, ambito Ambito) bool {
	if esEstandar(nombre) || comp.declaradaEsVisible(nombre, ambito) {
		return true
	}
	switch salida, produce := comp.variablesDeComandos[nombre]; {
	case produce && salida.laVe(ambito):
		return true
	case produce:
		probs.anotar(Variables, fichero, paso.Nombre, ambito.deUnAmbiente(), fmt.Sprintf("usa ${var.%s}, que es una "+
			"variable de salida del ámbito de un ambiente, y desde el ámbito compartido no se ve", nombre))
		return false
	}
	probs.anotar(Variables, fichero, paso.Nombre, ambito.deUnAmbiente(), fmt.Sprintf("usa ${var.%s}, que no es una "+
		"variable estándar, ni está declarada en un ámbito que se vea desde aquí, ni la produce ningún comando",
		nombre))
	return false
}

// tarde dice que algo se usa antes de producirse.
// necesita son las variables de salida que hay que haber producido para resolver un nombre: la que nombra, si
// es una variable de salida, y las que usan, una tras otra, los valores declarados por los que pasa. Un nombre
// declarado que además se produce cuenta como declarado, porque tiene valor desde el principio: que gane la
// producida cuando exista es precedencia, y eso es de Resolución.
func (comp *comprobacion) necesita(nombre string, ambito Ambito, visto map[string]bool) []variableDeComandoEnComprobacion {
	if visto[nombre] {
		return nil
	}
	visto[nombre] = true
	if esEstandar(nombre) {
		return nil
	}
	d, declarada := comp.declaradaVisible(nombre, ambito)
	if !declarada {
		if salida, produce := comp.variablesDeComandos[nombre]; produce {
			return []variableDeComandoEnComprobacion{salida}
		}
		return nil
	}
	var necesarias []variableDeComandoEnComprobacion
	nombres, _ := usos(d.Valor)
	for _, usado := range nombres {
		necesarias = append(necesarias, comp.necesita(usado, ambito, visto)...)
	}
	return necesarias
}

// circulos rechaza valores declarados que se usan entre ellos en círculo: ninguno se podría resolver. Se mira
// por ámbito, porque lo que se ve desde uno no se ve desde otro.
func (comp *comprobacion) circulos() {
	for _, ambito := range comp.ambitos() {
		comp.circulosEnUnAmbito(ambito)
	}
}

func (comp *comprobacion) circulosEnUnAmbito(ambito Ambito) {
	detector := &detectadorCirculos{
		comprobacion: comp,
		ambito:       ambito,
		literales:    make(map[string]variableDePipelineEnComprobacion),
		estado:       make(map[string]int),
		camino:       []string{},
	}
	detector.detectar()
}

type detectadorCirculos struct {
	comprobacion *comprobacion
	ambito       Ambito
	literales    map[string]variableDePipelineEnComprobacion
	estado       map[string]int
	camino       []string
}

const (
	sinVisitar = iota
	enCamino
	terminado
)

func (d *detectadorCirculos) detectar() {
	d.cargarLiterales()
	for _, variable := range d.comprobacion.variablesDePipeline {
		if d.esVariableNoVisitada(variable) {
			d.camino = d.camino[:0]
			d.visitar(variable.Nombre)
		}
	}
}

func (d *detectadorCirculos) cargarLiterales() {
	for _, v := range d.comprobacion.variablesDePipeline {
		if d.ambito.Ve(v.Ambito) {
			d.literales[v.Nombre] = v
		}
	}
}

func (d *detectadorCirculos) esVariableNoVisitada(v variableDePipelineEnComprobacion) bool {
	_, esLiteral := d.literales[v.Nombre]
	return esLiteral && d.estado[v.Nombre] == sinVisitar && v.Ambito == d.ambito
}

func (d *detectadorCirculos) visitar(nombre string) bool {
	d.estado[nombre] = enCamino
	d.camino = append(d.camino, nombre)

	nombres, _ := usos(d.literales[nombre].Valor)
	for _, usado := range nombres {
		if !d.esLiteralVisible(usado) {
			continue
		}

		switch d.estado[usado] {
		case enCamino:
			if d.debeReportarCirculo(usado) {
				d.reportarCirculo(nombre, usado)
			}
			return true
		case sinVisitar:
			if d.visitar(usado) {
				return true
			}
		}
	}

	d.camino = d.camino[:len(d.camino)-1]
	d.estado[nombre] = terminado
	return false
}

func (d *detectadorCirculos) esLiteralVisible(nombre string) bool {
	_, es := d.literales[nombre]
	return es
}

func (d *detectadorCirculos) debeReportarCirculo(usado string) bool {
	if d.ambito.EsCompartido() {
		return true
	}
	return tocaElAmbiente(d.caminoDesde(usado), d.literales)
}

func (d *detectadorCirculos) caminoDesde(nombre string) []string {
	inicio := slices.Index(d.camino, nombre)
	return d.camino[inicio:]
}

func (d *detectadorCirculos) reportarCirculo(nombre, usado string) {
	camino := d.caminoDesde(usado)
	d.comprobacion.falla(Variables, d.literales[nombre].fichero, "", d.ambito.deUnAmbiente(),
		"las variables se usan en círculo: %s → %s", strings.Join(camino, " → "), usado)
}

// tocaElAmbiente dice si un círculo pasa por alguna variable que no es compartida.
func tocaElAmbiente(circulo []string, literales map[string]variableDePipelineEnComprobacion) bool {
	return slices.ContainsFunc(circulo, func(n string) bool { return !literales[n].Ambito.EsCompartido() })
}

// problemas junta los fallos que se repiten en varios ambientes: si uno ocurre en todos, es un solo fallo sin
// ambiente.
type problemas struct {
	orden     []problema
	ambientes map[problema][]string
}

type problema struct {
	invariante    Invariante
	fichero, paso string
	detalle       string
}

func (p *problemas) anotar(inv Invariante, fichero, paso, ambiente, detalle string) {
	clave := problema{invariante: inv, fichero: fichero, paso: paso, detalle: detalle}
	if p.ambientes == nil {
		p.ambientes = map[problema][]string{}
	}
	if _, visto := p.ambientes[clave]; !visto {
		p.orden = append(p.orden, clave)
	}
	if !slices.Contains(p.ambientes[clave], ambiente) {
		p.ambientes[clave] = append(p.ambientes[clave], ambiente)
	}
}

func (p *problemas) reportar(comp *comprobacion, cuantosAmbientes int) {
	for _, clave := range p.orden {
		ambientes := p.ambientes[clave]
		if len(ambientes) == cuantosAmbientes || slices.Contains(ambientes, "") {
			comp.falla(clave.invariante, clave.fichero, clave.paso, "", "%s", clave.detalle)
			continue
		}
		for _, ambiente := range ambientes {
			comp.falla(clave.invariante, clave.fichero, clave.paso, ambiente, "%s", clave.detalle)
		}
	}
}

func contieneSinMayusculas(lista []string, s string) bool {
	return slices.ContainsFunc(lista, func(x string) bool { return strings.EqualFold(x, s) })
}
