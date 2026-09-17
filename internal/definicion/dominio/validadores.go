package dominio

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Validador define el contrato para validar un aspecto del pipeline.
type Validador interface {
	Validar(c *comprobacion) error
}

// validadoresDelPipeline son los validadores que Comprobar ejecuta después de ValidadorVersion, en este orden:
// cada uno asume que los anteriores corrieron, aunque hayan fallado (ninguno de ellos corta la comprobación,
// salvo ValidadorVersion). Son estructs vacíos y sin estado propio, así que compartir esta lista entre llamadas
// concurrentes a Comprobar es seguro.
var validadoresDelPipeline = []Validador{
	&ValidadorArchivosIlegibles{},
	&ValidadorAmbientes{},
	&ValidadorPasos{},
	&ValidadorSalidas{},
	&ValidadorVariablesDeclaradas{},
	&ValidadorUsos{},
}

// ResultadoValidacion centraliza el estado de la comprobación.
type ResultadoValidacion struct {
	Fallos               []Fallo
	AmbientesComprobados []AmbienteComprobado
	PasosComprobados     []PasoComprobado
	VariablesDePipeline  []variableDePipelineEnComprobacion
	VariablesDeComandos  map[string]variableDeComandoEnComprobacion
}

func (res *ResultadoValidacion) TieneErrores() bool {
	return len(res.Fallos) > 0
}

func (res *ResultadoValidacion) AlError() error {
	return &FallosDeComprobacion{Fallos: res.Fallos}
}

// variableDePipelineEnComprobacion es una variable y dónde se escribió.
type variableDePipelineEnComprobacion struct {
	VariableDePipelineComprobada
	fichero string
}

// posicion es dónde ocurre algo dentro del pipeline.
type posicion struct{ paso, comando int }

func (p posicion) antesDe(q posicion) bool {
	return p.paso < q.paso || (p.paso == q.paso && p.comando < q.comando)
}

// variableDeComandoEnComprobacion es una variable de salida y dónde se produce.
type variableDeComandoEnComprobacion struct {
	nombre     string
	donde      posicion
	paso       string
	compartida bool
}

func (s variableDeComandoEnComprobacion) laVe(ambito Ambito) bool {
	return s.compartida || !ambito.EsCompartido()
}

// ValidadorVersion valida la versión del formato.
type ValidadorVersion struct{}

func (val *ValidadorVersion) Validar(c *comprobacion) error {
	switch {
	case !c.pipelineDeclarado.Configuracion.Existe:
		c.falla(Formato, FileConfig, "", "", "no está, y el pipeline declara ahí su schema_version")
	case c.ilegible(FileConfig):
		c.falla(Formato, FileConfig, "", "", "%s", c.mapaIlegibles[FileConfig])
	case c.pipelineDeclarado.Configuracion.Datos.Version == nil:
		c.falla(Formato, FileConfig, "", "", "no dice schema_version, y la única que se lee es la %s", VersionDelFormato)
	case *c.pipelineDeclarado.Configuracion.Datos.Version != VersionDelFormato:
		c.falla(Formato, FileConfig, "", "", "schema_version %q no se lee: la única que se lee es la %s",
			*c.pipelineDeclarado.Configuracion.Datos.Version, VersionDelFormato)
	default:
		return nil
	}
	return c.resultado()
}

// ValidadorArchivosIlegibles reporta archivos que no se pudieron leer.
type ValidadorArchivosIlegibles struct{}

func (val *ValidadorArchivosIlegibles) Validar(c *comprobacion) error {
	for _, i := range c.pipelineDeclarado.Ilegibles {
		c.falla(Formato, i.Fichero, "", "", "%s", i.Motivo)
	}
	for _, ruta := range c.pipelineDeclarado.Desconocidos {
		c.falla(Formato, ruta, "", "", "no es parte del formato del pipeline")
	}
	if len(c.fallos) > 0 {
		return c.resultado()
	}
	return nil
}

// ValidadorAmbientes valida la declaración de ambientes.
type ValidadorAmbientes struct{}

func (val *ValidadorAmbientes) Validar(c *comprobacion) error {
	if !c.pipelineDeclarado.Ambientes.Existe {
		c.falla(Ambientes, FileEnvironments, "", "", "no está, y el pipeline declara ahí sus ambientes en orden")
		return c.resultado()
	}
	if len(c.pipelineDeclarado.Ambientes.Datos) == 0 && !c.ilegible(FileEnvironments) {
		c.falla(Ambientes, FileEnvironments, "", "", "no declara ningún ambiente")
		return c.resultado()
	}

	validador := &validadorAmbientesImpl{
		comprobacion: c,
		fichero:      FileEnvironments,
		nombres:      []string{},
		valores:      []string{},
	}
	return validador.validar()
}

type validadorAmbientesImpl struct {
	comprobacion *comprobacion
	fichero      string
	nombres      []string
	valores      []string
}

func (val *validadorAmbientesImpl) validar() error {
	for i, a := range val.comprobacion.pipelineDeclarado.Ambientes.Datos {
		val.validarUnAmbiente(i, a)
	}
	if len(val.comprobacion.fallos) > 0 {
		return val.comprobacion.resultado()
	}
	return nil
}

func (val *validadorAmbientesImpl) validarUnAmbiente(idx int, a AmbienteDeclarado) {
	donde := fmt.Sprintf("el ambiente %d", idx+1)
	if !val.esAmbienteValido(a, donde) {
		return
	}
	val.nombres = append(val.nombres, a.Nombre)
	val.valores = append(val.valores, a.Valor)
	val.comprobacion.ambientesComprobados = append(val.comprobacion.ambientesComprobados, AmbienteComprobado(a))
}

func (val *validadorAmbientesImpl) esAmbienteValido(a AmbienteDeclarado, donde string) bool {
	valido := true

	if a.Nombre == "" {
		val.comprobacion.falla(Formato, val.fichero, "", "", "%s no dice name", donde)
		valido = false
	}

	if !patronNombre.MatchString(a.Valor) {
		val.comprobacion.falla(Formato, val.fichero, "", "", "%s tiene value %q, que no sirve como directorio de variables/: "+
			"letras, dígitos, - y _", donde, a.Valor)
		valido = false
	}

	if contieneSinMayusculas(val.nombres, a.Nombre) {
		val.comprobacion.falla(Ambientes, val.fichero, "", "", "el name %q está dos veces", a.Nombre)
		valido = false
	}

	if contieneSinMayusculas(val.valores, a.Valor) {
		val.comprobacion.falla(Ambientes, val.fichero, "", "", "el value %q está dos veces", a.Valor)
		valido = false
	}

	return valido
}

// ValidadorPasos valida la declaración de pasos.
type ValidadorPasos struct{}

func (val *ValidadorPasos) Validar(c *comprobacion) error {
	if len(c.pipelineDeclarado.Pasos) == 0 {
		c.falla(Pasos, DirSteps, "", "", "el pipeline no tiene pasos")
		return c.resultado()
	}
	validador := &validadorPasosImpl{
		comprobacion: c,
		porOrden:     make(map[int]string),
		nombres:      []string{},
		consumidas:   make(map[string]bool),
	}
	if err := validador.validar(); err != nil {
		return err
	}
	validador.ordenar()
	return validador.validarPasosNoUsados()
}

type validadorPasosImpl struct {
	comprobacion *comprobacion
	porOrden     map[int]string
	nombres      []string
	consumidas   map[string]bool
}

func (val *validadorPasosImpl) validar() error {
	for _, escrito := range val.comprobacion.pipelineDeclarado.Pasos {
		val.validarUnPaso(escrito)
	}
	if len(val.comprobacion.fallos) > 0 {
		return val.comprobacion.resultado()
	}
	return nil
}

func (val *validadorPasosImpl) validarUnPaso(escrito PasoDeclarado) {
	directorio := DirSteps + escrito.Directorio
	matches := patronDirectorioDePaso.FindStringSubmatch(escrito.Directorio)
	if matches == nil {
		val.comprobacion.falla(Pasos, directorio, "", "", "el directorio de un paso se llama NN-<paso>, con NN de dos dígitos")
		return
	}

	orden, _ := strconv.Atoi(matches[1])
	nombre := matches[2]

	if !val.esPasoValido(directorio, nombre, orden, matches[1]) {
		return
	}

	val.procesarPaso(escrito, nombre, orden)
}

func (val *validadorPasosImpl) esPasoValido(directorio, nombre string, orden int, ordenStr string) bool {
	if !patronNombre.MatchString(nombre) {
		val.comprobacion.falla(Pasos, directorio, "", "", "%q no sirve como nombre de un paso: letras, dígitos, - y _", nombre)
		return false
	}

	if otro, repetido := val.porOrden[orden]; repetido {
		val.comprobacion.falla(Pasos, directorio, nombre, "", "el orden %s ya es de steps/%s", ordenStr, otro)
		return false
	}

	if contieneSinMayusculas(val.nombres, nombre) {
		val.comprobacion.falla(Pasos, directorio, nombre, "", "otro paso ya se llama %q", nombre)
		return false
	}

	return true
}

func (val *validadorPasosImpl) procesarPaso(escrito PasoDeclarado, nombre string, orden int) {
	val.porOrden[orden] = escrito.Directorio
	val.nombres = append(val.nombres, nombre)
	val.consumidas[nombre] = true

	paso := PasoComprobado{Nombre: nombre, Orden: orden}
	configuracion, tiene := val.comprobacion.pipelineDeclarado.Configuracion.Datos.Pasos[nombre]
	if tiene {
		val.comprobacion.configuracion(&paso, &configuracion)
	} else {
		val.comprobacion.configuracion(&paso, nil)
	}
	val.comprobacion.material(&paso, escrito.Material)
	val.comprobacion.comandos(&paso, escrito.Comandos)
	val.comprobacion.pasosComprobados = append(val.comprobacion.pasosComprobados, paso)
}

func (val *validadorPasosImpl) ordenar() {
	sort.Slice(val.comprobacion.pasosComprobados, func(i, j int) bool {
		return val.comprobacion.pasosComprobados[i].Orden < val.comprobacion.pasosComprobados[j].Orden
	})
}

func (val *validadorPasosImpl) validarPasosNoUsados() error {
	var sinPaso []string
	for nombre := range val.comprobacion.pipelineDeclarado.Configuracion.Datos.Pasos {
		if !val.consumidas[nombre] {
			sinPaso = append(sinPaso, nombre)
		}
	}
	sort.Strings(sinPaso)
	for _, nombre := range sinPaso {
		val.comprobacion.falla(Pasos, FileConfig, "", "", "declara la configuración de %q, que no es un paso: no hay "+
			"ningún steps/NN-%s/", nombre, nombre)
	}
	if len(val.comprobacion.fallos) > 0 {
		return val.comprobacion.resultado()
	}
	return nil
}

// ValidadorSalidas valida dónde se produce cada variable de salida. Una variable de salida pertenece a un
// ámbito y no a un paso, así que dos comandos del mismo ámbito no pueden producir el mismo nombre: quien lo
// usara no sabría cuál de los dos ve.
type ValidadorSalidas struct{}

func (val *ValidadorSalidas) Validar(c *comprobacion) error {
	c.variablesDeComandos = map[string]variableDeComandoEnComprobacion{}
	validador := &validadorSalidasImpl{comprobacion: c}
	validador.validar()
	if len(c.fallos) > 0 {
		return c.resultado()
	}
	return nil
}

type validadorSalidasImpl struct {
	comprobacion *comprobacion
}

func (vs *validadorSalidasImpl) validar() {
	for i, paso := range vs.comprobacion.pasosComprobados {
		vs.validarPaso(i, paso)
	}
}

func (vs *validadorSalidasImpl) validarPaso(idxPaso int, paso PasoComprobado) {
	for j, comando := range paso.Comandos {
		vs.validarComando(idxPaso, j, paso, comando)
	}
}

func (vs *validadorSalidasImpl) validarComando(idxPaso, idxCmd int, paso PasoComprobado, comando ComandoComprobado) {
	for _, salida := range comando.VariablesDeSalida {
		vs.registrarSalida(idxPaso, idxCmd, paso, salida)
	}
}

func (vs *validadorSalidasImpl) registrarSalida(idxPaso, idxCmd int, paso PasoComprobado, salida VariableDeComandoComprobada) {
	anterior, existia := vs.comprobacion.variablesDeComandos[salida.Nombre]
	if !existia {
		vs.comprobacion.variablesDeComandos[salida.Nombre] = variableDeComandoEnComprobacion{
			nombre: salida.Nombre, donde: posicion{paso: idxPaso, comando: idxCmd}, paso: paso.Nombre, compartida: salida.Ambito != nil,
		}
		return
	}
	vs.validarDuplicado(idxCmd, paso, salida, anterior)
}

func (vs *validadorSalidasImpl) validarDuplicado(idxCmd int, paso PasoComprobado, salida VariableDeComandoComprobada, anterior variableDeComandoEnComprobacion) {
	fichero := paso.Directorio() + "/commands.yaml"

	if anterior.paso == paso.Nombre && anterior.donde.comando == idxCmd {
		return
	}

	if anterior.compartida != (salida.Ambito != nil) {
		vs.comprobacion.falla(VariablesDeSalida, fichero, paso.Nombre, "", "el comando %d produce %q, que el comando %d "+
			"de %q produce en el otro ámbito: un nombre pertenece a un solo ámbito", idxCmd+1, salida.Nombre,
			anterior.donde.comando+1, anterior.paso)
		return
	}

	vs.comprobacion.falla(VariablesDeSalida, fichero, paso.Nombre, "", "el comando %d produce %q, que ya produce el "+
		"comando %d de %q: en un ámbito, una variable de salida se produce en un solo sitio", idxCmd+1,
		salida.Nombre, anterior.donde.comando+1, anterior.paso)
}

// ValidadorVariablesDeclaradas pone cada fichero de variables/ en su ámbito: variables/<ambiente>/ declara las
// del ámbito de ese ambiente, y la raíz de variables/, las del ámbito compartido (IT-12 DEC-12.7). El nombre
// del fichero solo organiza: las variables son del ámbito, y las ve todo paso que se ejecuta en él.
type ValidadorVariablesDeclaradas struct{}

func (val *ValidadorVariablesDeclaradas) Validar(c *comprobacion) error {
	validador := &validadorVariablesDeclaradasImpl{
		comprobacion: c,
		// donde recuerda en qué ámbitos ya está declarado cada nombre, para ver los que chocan.
		donde: map[string][]variableDePipelineEnComprobacion{},
	}
	validador.validar()
	if len(c.fallos) > 0 {
		return c.resultado()
	}
	return nil
}

type validadorVariablesDeclaradasImpl struct {
	comprobacion *comprobacion
	donde        map[string][]variableDePipelineEnComprobacion
}

func (vv *validadorVariablesDeclaradasImpl) validar() {
	for _, escritas := range vv.comprobacion.pipelineDeclarado.Variables {
		vv.validarFichero(escritas)
	}
}

func (vv *validadorVariablesDeclaradasImpl) validarFichero(escritas VariablesDePipelineDeclarada) {
	fichero := escritas.Fichero
	ambito := Compartido
	if escritas.Ambito != "" {
		ambito = Ambito(escritas.Ambito)
	}
	if !ambito.EsCompartido() && !slices.ContainsFunc(vv.comprobacion.ambientesComprobados,
		func(a AmbienteComprobado) bool { return a.Valor == escritas.Ambito }) {
		vv.comprobacion.falla(Variables, fichero, "", "", DirVariables+"%s/ no es de ningún ámbito: un directorio de "+DirVariables+
			"es el value de un ambiente, y las variables compartidas van en la raíz", escritas.Ambito)
		return
	}
	for _, variable := range escritas.Variables {
		vv.procesarVariable(variable, fichero, ambito)
	}
}

func (vv *validadorVariablesDeclaradasImpl) procesarVariable(variable VariableDePipelineDeclarada, fichero string, ambito Ambito) {
	if !vv.nombreDeclarable(fichero, ambito, variable.Nombre) {
		return
	}
	if otra, choca := choque(vv.donde[variable.Nombre], ambito); choca {
		if otra.Ambito == ambito {
			vv.comprobacion.falla(Variables, fichero, "", ambito.deUnAmbiente(), "la variable %q ya está declarada en %s",
				variable.Nombre, otra.fichero)
		} else {
			vv.comprobacion.falla(Variables, fichero, "", "", "la variable %q ya está declarada en %s, que es del ámbito %s, "+
				"y un paso ve los dos: un nombre pertenece a un solo ámbito", variable.Nombre, otra.fichero, otra.Ambito)
		}
		return
	}
	nueva := variableDePipelineEnComprobacion{VariableDePipelineComprobada: vv.variable(fichero, ambito, variable), fichero: fichero}
	vv.donde[variable.Nombre] = append(vv.donde[variable.Nombre], nueva)
	vv.comprobacion.variablesDePipeline = append(vv.comprobacion.variablesDePipeline, nueva)
}

func (vv *validadorVariablesDeclaradasImpl) nombreDeclarable(fichero string, ambito Ambito, nombre string) bool {
	switch {
	case nombre == "":
		vv.comprobacion.falla(Formato, fichero, "", ambito.deUnAmbiente(), "una variable no dice name")
	case !patronVariable.MatchString(nombre):
		vv.comprobacion.falla(Formato, fichero, "", ambito.deUnAmbiente(), "%q no es un nombre de variable", nombre)
	case esEstandar(nombre):
		vv.comprobacion.falla(Variables, fichero, "", ambito.deUnAmbiente(), "%q es una variable estándar: el motor la da siempre, "+
			"y no se declara", nombre)
	default:
		return true
	}
	return false
}

// variable lee el valor escrito. Si no está, la variable queda sin valor: lo que la usa no suma un «no
// declarada» que no es verdad, y con el fallo no hay pipeline.
func (vv *validadorVariablesDeclaradasImpl) variable(fichero string, ambito Ambito, variable VariableDePipelineDeclarada) VariableDePipelineComprobada {
	declarada := VariableDePipelineComprobada{Nombre: variable.Nombre, Descripcion: variable.Descripcion, Ambito: ambito}
	if variable.Valor == nil {
		vv.comprobacion.falla(Formato, fichero, "", ambito.deUnAmbiente(), "la variable %q no dice value", variable.Nombre)
		return declarada
	}
	declarada.Valor = *variable.Valor
	return declarada
}

// ValidadorUsos comprueba que toda variable usada en un comando, en una plantilla o en un valor declarado sea
// una variable estándar, esté declarada en un ámbito que se vea desde donde se usa, o la produzca un comando; y
// que lo que necesita esté producido antes de usarse (IT-02 DEC-02.7).
//
// Quien se ejecuta en un ambiente ve: las variables estándar; las declaradas en el ámbito de ese ambiente y en
// el compartido; y las variables de salida producidas antes, tanto por los comandos anteriores del mismo paso
// como por los pasos anteriores. Lo compartido no ve lo del ambiente: un valor compartido que dependiera de un
// ambiente dejaría de ser el mismo en todos.
type ValidadorUsos struct{}

func (val *ValidadorUsos) Validar(c *comprobacion) error {
	validador := &validadorUsosImpl{comprobacion: c}
	validador.circulos()
	validador.valoresDeclarados()
	validador.usosEnLosPasos()
	if len(c.fallos) > 0 {
		return c.resultado()
	}
	return nil
}

type validadorUsosImpl struct {
	comprobacion *comprobacion
}

// circulos rechaza valores declarados que se usan entre ellos en círculo: ninguno se podría resolver. Se mira
// por ámbito, porque lo que se ve desde uno no se ve desde otro.
func (vu *validadorUsosImpl) circulos() {
	for _, ambito := range vu.ambitos() {
		vu.circulosEnUnAmbito(ambito)
	}
}

// ambitos son aquellos en los que se declara algo: el compartido y el de cada ambiente.
func (vu *validadorUsosImpl) ambitos() []Ambito {
	ambitos := []Ambito{Compartido}
	for _, a := range vu.comprobacion.ambientesComprobados {
		ambitos = append(ambitos, Ambito(a.Valor))
	}
	return ambitos
}

func (vu *validadorUsosImpl) circulosEnUnAmbito(ambito Ambito) {
	detector := &detectadorCirculos{
		comprobacion: vu.comprobacion,
		ambito:       ambito,
		literales:    make(map[string]variableDePipelineEnComprobacion),
		estado:       make(map[string]int),
		camino:       []string{},
	}
	detector.detectar()
}

// valoresDeclarados comprueba lo que usa cada valor declarado, sin mirar el orden: cuándo se resuelve una
// variable depende de dónde se use, y eso se mira en usosEnLosPasos.
func (vu *validadorUsosImpl) valoresDeclarados() {
	for _, variable := range vu.comprobacion.variablesDePipeline {
		nombres, malformados := usos(variable.Valor)
		ambiente := variable.Ambito.deUnAmbiente()
		for _, nombre := range malformados {
			vu.comprobacion.falla(Formato, variable.fichero, "", ambiente, "${var.%s} no es un nombre de variable", nombre)
		}
		for _, nombre := range nombres {
			switch salida, produce := vu.comprobacion.variablesDeComandos[nombre]; {
			case esEstandar(nombre) || vu.declaradaEsVisible(nombre, variable.Ambito):
			case produce && salida.laVe(variable.Ambito):
			case produce:
				vu.comprobacion.falla(Variables, variable.fichero, "", ambiente, "%q usa ${var.%s}, que es una variable de salida del "+
					"ámbito de un ambiente, y desde el ámbito compartido no se ve", variable.Nombre, nombre)
			default:
				vu.comprobacion.falla(Variables, variable.fichero, "", ambiente, "%q usa ${var.%s}, que no es una variable estándar, ni "+
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
func (vu *validadorUsosImpl) usosEnLosPasos() {
	probs := &problemas{}
	for _, a := range vu.comprobacion.ambientesComprobados {
		vu.usosEnUnAmbiente(probs, Ambito(a.Valor))
	}
	probs.reportar(vu.comprobacion, len(vu.comprobacion.ambientesComprobados))
}

func (vu *validadorUsosImpl) usosEnUnAmbiente(probs *problemas, ambiente Ambito) {
	for i, paso := range vu.comprobacion.pasosComprobados {
		ambito := ambiente
		if paso.Ambito != nil {
			ambito = *paso.Ambito
		}
		vu.usosEnUnPaso(probs, i, paso, ambito)
	}
}

func (vu *validadorUsosImpl) usosEnUnPaso(probs *problemas, idxPaso int, paso PasoComprobado, ambito Ambito) {
	for j, comando := range paso.Comandos {
		punto := posicion{paso: idxPaso, comando: j}
		vu.revisar(probs, paso, paso.Directorio()+"/commands.yaml", comando.Linea, ambito, punto)
		vu.revisarPlantillas(probs, paso, idxPaso, j, comando, ambito)
	}
}

func (vu *validadorUsosImpl) revisarPlantillas(probs *problemas, paso PasoComprobado, idxPaso, idxCmd int, comando ComandoComprobado, ambito Ambito) {
	for _, ruta := range comando.Plantillas {
		k := slices.IndexFunc(paso.Material, func(f FicheroComprobado) bool { return f.Ruta == ruta })
		if k >= 0 {
			punto := posicion{paso: idxPaso, comando: idxCmd}
			vu.revisar(probs, paso, paso.Directorio()+"/"+ruta, paso.Material[k].Contenido, ambito, punto)
		}
	}
}

// revisar mira un texto que se interpola en un punto del pipeline: cada nombre que usa tiene que verse desde
// ese ámbito, y lo que hace falta para resolverlo tiene que estar producido antes.
func (vu *validadorUsosImpl) revisar(probs *problemas, paso PasoComprobado, fichero, texto string, ambito Ambito, punto posicion) {
	nombres, malformados := usos(texto)
	for _, nombre := range malformados {
		probs.anotar(Formato, fichero, paso.Nombre, "", fmt.Sprintf("${var.%s} no es un nombre de variable", nombre))
	}
	for _, nombre := range nombres {
		if !vu.seVe(probs, paso, fichero, nombre, ambito) {
			continue
		}
		for _, necesaria := range vu.necesita(nombre, ambito, map[string]bool{}) {
			if necesaria.donde.antesDe(punto) {
				continue
			}
			probs.anotar(Variables, fichero, paso.Nombre, ambito.deUnAmbiente(), tarde(nombre, necesaria, punto))
		}
	}
}

// seVe dice si un nombre usado dentro de un paso se ve desde su ámbito, y si no, lo anota: el orden lo mira
// revisar, no esto. Una variable de salida solo se ve si su ámbito la deja ver desde aquí
// (variableDeComandoEnComprobacion.laVe): un paso de scope: shared ve las suyas, no las de un ambiente.
func (vu *validadorUsosImpl) seVe(probs *problemas, paso PasoComprobado, fichero, nombre string, ambito Ambito) bool {
	if esEstandar(nombre) || vu.declaradaEsVisible(nombre, ambito) {
		return true
	}
	switch salida, produce := vu.comprobacion.variablesDeComandos[nombre]; {
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

// necesita son las variables de salida que hay que haber producido para resolver un nombre: la que nombra, si
// es una variable de salida, y las que usan, una tras otra, los valores declarados por los que pasa. Un nombre
// declarado que además se produce cuenta como declarado, porque tiene valor desde el principio: que gane la
// producida cuando exista es precedencia, y eso es de Resolución.
func (vu *validadorUsosImpl) necesita(nombre string, ambito Ambito, visto map[string]bool) []variableDeComandoEnComprobacion {
	if visto[nombre] {
		return nil
	}
	visto[nombre] = true
	if esEstandar(nombre) {
		return nil
	}
	declarada, esVisible := vu.declaradaVisible(nombre, ambito)
	if !esVisible {
		if salida, produce := vu.comprobacion.variablesDeComandos[nombre]; produce {
			return []variableDeComandoEnComprobacion{salida}
		}
		return nil
	}
	var necesarias []variableDeComandoEnComprobacion
	nombres, _ := usos(declarada.Valor)
	for _, usado := range nombres {
		necesarias = append(necesarias, vu.necesita(usado, ambito, visto)...)
	}
	return necesarias
}

// declaradaVisible busca un nombre entre las variables declaradas que se ven desde un ámbito.
func (vu *validadorUsosImpl) declaradaVisible(nombre string, ambito Ambito) (variableDePipelineEnComprobacion, bool) {
	for _, variable := range vu.comprobacion.variablesDePipeline {
		if variable.Nombre == nombre && ambito.Ve(variable.Ambito) {
			return variable, true
		}
	}
	return variableDePipelineEnComprobacion{}, false
}

// declaradaEsVisible es declaradaVisible cuando solo hace falta saber si está.
func (vu *validadorUsosImpl) declaradaEsVisible(nombre string, ambito Ambito) bool {
	_, hay := vu.declaradaVisible(nombre, ambito)
	return hay
}

// choque dice si un nombre ya declarado choca con este ámbito. Dos ambientes distintos no chocan, porque no se
// ven a la vez; el ámbito compartido choca con todos, porque se ve desde todos.
func choque(previas []variableDePipelineEnComprobacion, ambito Ambito) (variableDePipelineEnComprobacion, bool) {
	for _, previa := range previas {
		if previa.Ambito.Ve(ambito) || ambito.Ve(previa.Ambito) {
			return previa, true
		}
	}
	return variableDePipelineEnComprobacion{}, false
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

func (probs *problemas) anotar(inv Invariante, fichero, paso, ambiente, detalle string) {
	clave := problema{invariante: inv, fichero: fichero, paso: paso, detalle: detalle}
	if probs.ambientes == nil {
		probs.ambientes = map[problema][]string{}
	}
	if _, visto := probs.ambientes[clave]; !visto {
		probs.orden = append(probs.orden, clave)
	}
	if !slices.Contains(probs.ambientes[clave], ambiente) {
		probs.ambientes[clave] = append(probs.ambientes[clave], ambiente)
	}
}

func (probs *problemas) reportar(comp *comprobacion, cuantosAmbientes int) {
	for _, clave := range probs.orden {
		ambientes := probs.ambientes[clave]
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

func tarde(nombre string, necesaria variableDeComandoEnComprobacion, punto posicion) string {
	cuando := fmt.Sprintf("la produce el comando %d de %q, que va después", necesaria.donde.comando+1, necesaria.paso)
	if necesaria.donde == punto {
		cuando = "la produce este mismo comando"
	}
	if necesaria.nombre == nombre {
		return fmt.Sprintf("usa ${var.%s}, y %s", nombre, cuando)
	}
	return fmt.Sprintf("usa ${var.%s}, que necesita la salida %q, y %s", nombre, necesaria.nombre, cuando)
}
