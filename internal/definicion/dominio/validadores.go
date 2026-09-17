package dominio

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
)

// Validador define el contrato para validar un aspecto del pipeline.
type Validador interface {
	Validar(c *comprobacion) error
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
		for _, i := range c.pipelineDeclarado.Ilegibles {
			if i.Fichero == FileConfig {
				c.falla(Formato, FileConfig, "", "", "%s", i.Motivo)
			}
		}
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
	m := patronDirectorioDePaso.FindStringSubmatch(escrito.Directorio)
	if m == nil {
		val.comprobacion.falla(Pasos, directorio, "", "", "el directorio de un paso se llama NN-<paso>, con NN de dos dígitos")
		return
	}

	orden, _ := strconv.Atoi(m[1])
	nombre := m[2]

	if !val.esPasoValido(directorio, nombre, orden, m[1]) {
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
	val.comprobacion.consumidas[nombre] = true
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

// ValidadorSalidas valida dónde se produce cada variable de salida.
type ValidadorSalidas struct{}

func (val *ValidadorSalidas) Validar(c *comprobacion) error {
	c.variablesDeComandos = map[string]variableDeComandoEnComprobacion{}
	for i, paso := range c.pasosComprobados {
		fichero := paso.Directorio() + "/commands.yaml"
		for j, comando := range paso.Comandos {
			for _, s := range comando.VariablesDeSalida {
				anterior, repetida := c.variablesDeComandos[s.Nombre]
				switch {
				case !repetida:
					c.variablesDeComandos[s.Nombre] = variableDeComandoEnComprobacion{
						nombre: s.Nombre, donde: posicion{paso: i, comando: j}, paso: paso.Nombre, compartida: s.Ambito != nil,
					}
				case anterior.paso == paso.Nombre && anterior.donde.comando == j:
				case anterior.compartida != (s.Ambito != nil):
					c.falla(VariablesDeSalida, fichero, paso.Nombre, "", "el comando %d produce %q, que el comando %d "+
						"de %q produce en el otro ámbito: un nombre pertenece a un solo ámbito", j+1, s.Nombre,
						anterior.donde.comando+1, anterior.paso)
				default:
					c.falla(VariablesDeSalida, fichero, paso.Nombre, "", "el comando %d produce %q, que ya produce el "+
						"comando %d de %q: en un ámbito, una variable de salida se produce en un solo sitio", j+1,
						s.Nombre, anterior.donde.comando+1, anterior.paso)
				}
			}
		}
	}
	if len(c.fallos) > 0 {
		return c.resultado()
	}
	return nil
}

// ValidadorVariablesDeclaradas valida la declaración de variables.
type ValidadorVariablesDeclaradas struct{}

func (val *ValidadorVariablesDeclaradas) Validar(c *comprobacion) error {
	donde := map[string][]variableDePipelineEnComprobacion{}
	for _, escritas := range c.pipelineDeclarado.Variables {
		fichero := escritas.Fichero
		ambito := Compartido
		if escritas.Ambito != "" {
			ambito = Ambito(escritas.Ambito)
		}
		if !ambito.EsCompartido() && !slices.ContainsFunc(c.ambientesComprobados,
			func(a AmbienteComprobado) bool { return a.Valor == escritas.Ambito }) {
			c.falla(Variables, fichero, "", "", DirVariables+"%s/ no es de ningún ámbito: un directorio de "+DirVariables+
				"es el value de un ambiente, y las variables compartidas van en la raíz", escritas.Ambito)
			continue
		}
		for _, variable := range escritas.Variables {
			val.procesarVariable(c, variable, fichero, ambito, donde)
		}
	}
	if len(c.fallos) > 0 {
		return c.resultado()
	}
	return nil
}

func (val *ValidadorVariablesDeclaradas) procesarVariable(c *comprobacion, variable VariableDePipelineDeclarada, fichero string, ambito Ambito, donde map[string][]variableDePipelineEnComprobacion) {
	if !val.nombreDeclarable(c, fichero, ambito, variable.Nombre) {
		return
	}
	if otra, choca := choque(donde[variable.Nombre], ambito); choca {
		if otra.Ambito == ambito {
			c.falla(Variables, fichero, "", ambito.deUnAmbiente(), "la variable %q ya está declarada en %s",
				variable.Nombre, otra.fichero)
		} else {
			c.falla(Variables, fichero, "", "", "la variable %q ya está declarada en %s, que es del ámbito %s, "+
				"y un paso ve los dos: un nombre pertenece a un solo ámbito", variable.Nombre, otra.fichero, otra.Ambito)
		}
		return
	}
	nueva := variableDePipelineEnComprobacion{VariableDePipelineComprobada: val.variable(c, fichero, ambito, variable), fichero: fichero}
	donde[variable.Nombre] = append(donde[variable.Nombre], nueva)
	c.variablesDePipeline = append(c.variablesDePipeline, nueva)
}

func (val *ValidadorVariablesDeclaradas) nombreDeclarable(c *comprobacion, fichero string, ambito Ambito, nombre string) bool {
	switch {
	case nombre == "":
		c.falla(Formato, fichero, "", ambito.deUnAmbiente(), "una variable no dice name")
	case !patronVariable.MatchString(nombre):
		c.falla(Formato, fichero, "", ambito.deUnAmbiente(), "%q no es un nombre de variable", nombre)
	case esEstandar(nombre):
		c.falla(Variables, fichero, "", ambito.deUnAmbiente(), "%q es una variable estándar: el motor la da siempre, "+
			"y no se declara", nombre)
	default:
		return true
	}
	return false
}

func (val *ValidadorVariablesDeclaradas) variable(c *comprobacion, fichero string, ambito Ambito, variable VariableDePipelineDeclarada) VariableDePipelineComprobada {
	declarada := VariableDePipelineComprobada{Nombre: variable.Nombre, Descripcion: variable.Descripcion, Ambito: ambito}
	if variable.Valor == nil {
		c.falla(Formato, fichero, "", ambito.deUnAmbiente(), "la variable %q no dice value", variable.Nombre)
		return declarada
	}
	declarada.Valor = *variable.Valor
	return declarada
}

// ValidadorUsos valida los usos de variables.
type ValidadorUsos struct{}

func (val *ValidadorUsos) Validar(c *comprobacion) error {
	val.circulos(c)
	if len(c.fallos) > 0 {
		return c.resultado()
	}
	val.valoresDeclarados(c)
	if len(c.fallos) > 0 {
		return c.resultado()
	}
	val.usosEnLosPasos(c)
	if len(c.fallos) > 0 {
		return c.resultado()
	}
	return nil
}

func (val *ValidadorUsos) circulos(c *comprobacion) {
	for _, ambito := range val.ambitos(c) {
		val.circulosEnUnAmbito(c, ambito)
	}
}

func (val *ValidadorUsos) ambitos(c *comprobacion) []Ambito {
	ambitos := []Ambito{Compartido}
	for _, a := range c.ambientesComprobados {
		ambitos = append(ambitos, Ambito(a.Valor))
	}
	return ambitos
}

func (val *ValidadorUsos) valoresDeclarados(c *comprobacion) {
	for _, variable := range c.variablesDePipeline {
		nombres, malformados := usos(variable.Valor)
		ambiente := variable.Ambito.deUnAmbiente()
		for _, nombre := range malformados {
			c.falla(Formato, variable.fichero, "", ambiente, "${var.%s} no es un nombre de variable", nombre)
		}
		for _, nombre := range nombres {
			switch salida, produce := c.variablesDeComandos[nombre]; {
			case esEstandar(nombre) || val.declaradaEsVisible(c, nombre, variable.Ambito):
			case produce && salida.laVe(variable.Ambito):
			case produce:
				c.falla(Variables, variable.fichero, "", ambiente, "%q usa ${var.%s}, que es una variable de salida del "+
					"ámbito de un ambiente, y desde el ámbito compartido no se ve", variable.Nombre, nombre)
			default:
				c.falla(Variables, variable.fichero, "", ambiente, "%q usa ${var.%s}, que no es una variable estándar, ni "+
					"está declarada en un ámbito que se vea desde aquí, ni la produce ningún comando", variable.Nombre, nombre)
			}
		}
	}
}

func (val *ValidadorUsos) usosEnLosPasos(c *comprobacion) {
	p := &problemas{}
	for _, a := range c.ambientesComprobados {
		val.usosEnUnAmbiente(c, p, Ambito(a.Valor))
	}
	p.reportar(c, len(c.ambientesComprobados))
}

func (val *ValidadorUsos) usosEnUnAmbiente(c *comprobacion, p *problemas, ambiente Ambito) {
	for i, paso := range c.pasosComprobados {
		ambito := ambiente
		if paso.Ambito != nil {
			ambito = *paso.Ambito
		}
		val.usosEnUnPaso(c, p, i, paso, ambito)
	}
}

func (val *ValidadorUsos) usosEnUnPaso(c *comprobacion, p *problemas, idxPaso int, paso PasoComprobado, ambito Ambito) {
	for j, comando := range paso.Comandos {
		punto := posicion{paso: idxPaso, comando: j}
		val.revisar(c, p, paso, paso.Directorio()+"/commands.yaml", comando.Linea, ambito, punto)
		val.revisarPlantillas(c, p, paso, idxPaso, j, comando, ambito)
	}
}

func (val *ValidadorUsos) revisarPlantillas(c *comprobacion, p *problemas, paso PasoComprobado, idxPaso, idxCmd int, comando ComandoComprobado, ambito Ambito) {
	for _, ruta := range comando.Plantillas {
		k := slices.IndexFunc(paso.Material, func(f FicheroComprobado) bool { return f.Ruta == ruta })
		if k >= 0 {
			punto := posicion{paso: idxPaso, comando: idxCmd}
			val.revisar(c, p, paso, paso.Directorio()+"/"+ruta, paso.Material[k].Contenido, ambito, punto)
		}
	}
}

func (val *ValidadorUsos) revisar(c *comprobacion, p *problemas, paso PasoComprobado, fichero, texto string, ambito Ambito, punto posicion) {
	nombres, malformados := usos(texto)
	for _, nombre := range malformados {
		p.anotar(Formato, fichero, paso.Nombre, "", fmt.Sprintf("${var.%s} no es un nombre de variable", nombre))
	}
	for _, nombre := range nombres {
		if !val.seVe(c, p, paso, fichero, nombre, ambito) {
			continue
		}
		for _, necesaria := range val.necesita(c, nombre, ambito, map[string]bool{}) {
			if necesaria.donde.antesDe(punto) {
				continue
			}
			p.anotar(Variables, fichero, paso.Nombre, ambito.deUnAmbiente(), tarde(nombre, necesaria, punto))
		}
	}
}

func (val *ValidadorUsos) seVe(c *comprobacion, p *problemas, paso PasoComprobado, fichero, nombre string, ambito Ambito) bool {
	if esEstandar(nombre) || val.declaradaEsVisible(c, nombre, ambito) {
		return true
	}
	switch salida, produce := c.variablesDeComandos[nombre]; {
	case produce && salida.laVe(ambito):
		return true
	case produce:
		p.anotar(Variables, fichero, paso.Nombre, ambito.deUnAmbiente(), fmt.Sprintf("usa ${var.%s}, que es una "+
			"variable de salida del ámbito de un ambiente, y desde el ámbito compartido no se ve", nombre))
		return false
	}
	p.anotar(Variables, fichero, paso.Nombre, ambito.deUnAmbiente(), fmt.Sprintf("usa ${var.%s}, que no es una "+
		"variable estándar, ni está declarada en un ámbito que se vea desde aquí, ni la produce ningún comando",
		nombre))
	return false
}

func (val *ValidadorUsos) necesita(c *comprobacion, nombre string, ambito Ambito, visto map[string]bool) []variableDeComandoEnComprobacion {
	if visto[nombre] {
		return nil
	}
	visto[nombre] = true
	if esEstandar(nombre) {
		return nil
	}
	d, declarada := val.declaradaVisible(c, nombre, ambito)
	if !declarada {
		if salida, produce := c.variablesDeComandos[nombre]; produce {
			return []variableDeComandoEnComprobacion{salida}
		}
		return nil
	}
	var necesarias []variableDeComandoEnComprobacion
	nombres, _ := usos(d.Valor)
	for _, usado := range nombres {
		necesarias = append(necesarias, val.necesita(c, usado, ambito, visto)...)
	}
	return necesarias
}

func (val *ValidadorUsos) declaradaVisible(c *comprobacion, nombre string, ambito Ambito) (variableDePipelineEnComprobacion, bool) {
	for _, variable := range c.variablesDePipeline {
		if variable.Nombre == nombre && ambito.Ve(variable.Ambito) {
			return variable, true
		}
	}
	return variableDePipelineEnComprobacion{}, false
}

func (val *ValidadorUsos) declaradaEsVisible(c *comprobacion, nombre string, ambito Ambito) bool {
	_, hay := val.declaradaVisible(c, nombre, ambito)
	return hay
}

func (val *ValidadorUsos) circulosEnUnAmbito(c *comprobacion, ambito Ambito) {
	detector := &detectadorCirculos{
		comprobacion: c,
		ambito:       ambito,
		literales:    make(map[string]variableDePipelineEnComprobacion),
		estado:       make(map[string]int),
		camino:       []string{},
	}
	detector.detectar()
}

func choque(previas []variableDePipelineEnComprobacion, ambito Ambito) (variableDePipelineEnComprobacion, bool) {
	for _, p := range previas {
		if p.Ambito.Ve(ambito) || ambito.Ve(p.Ambito) {
			return p, true
		}
	}
	return variableDePipelineEnComprobacion{}, false
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
