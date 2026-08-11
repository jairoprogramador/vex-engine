package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

// DeclarationVersion identifica la regla de la huella de la DECLARACIÓN de un
// step: `pipe-v1`.
//
// Sustituye a `inst-v1` y a `vars-v1`, que se retiran con la spec 27. No es una
// regla más: es una sola respuesta a una sola pregunta —«¿qué declara este
// step?»— donde antes había dos que había que acordarse de componer, y un
// tercio del material —el árbol del directorio del step— que no estaba en
// ninguna de las dos (D14).
//
// El token es propio y distinto del `v1:` del árbol por la misma razón de
// siempre: las dos reglas comparten tipo y una compone sobre la otra, pero sus
// versiones tienen que poder moverse por separado.
const DeclarationVersion = "pipe-v1"

const (
	// declarationHeader encabeza el material. Existe para que esta huella no
	// pueda coincidir con el hash de otra cosa que se le parezca.
	declarationHeader = "vex-step-declaration/" + DeclarationVersion

	// fieldSep separa los campos dentro de una línea del material.
	//
	// Es U+001E (RS) y no `:` como en la huella del árbol porque aquí todos los
	// valores textuales viajan entre comillas de `strconv.Quote`, que escapa
	// cualquier carácter de control: ni el separador ni el salto de línea pueden
	// aparecer dentro de un campo. Esa es la razón por la que esta regla NO
	// hereda la ambigüedad teórica del separador que la huella del árbol se dejó
	// congelada (SPEC-v1.md §6).
	fieldSep = "\x1e"
)

// declarationFileNames son los archivos del directorio de un step que el motor
// SABE INTERPRETAR, y que por tanto entran NORMALIZADOS y no como parte del
// árbol crudo (spec 27 §5.2 y §5.3).
//
// Es normativo y está en `SPEC-PIPELINE-v1.md` §3.4: la lista decide el reparto
// entre las dos mitades del material, así que quitar o añadir un nombre es
// cambiar la regla. Vive aquí —y no en el repositorio que lee esos archivos—
// porque una implementación independiente tiene que poder reproducir la huella
// sin conocer la infraestructura de este motor.
//
// El tercer archivo de declaración, `variables/<ambiente>/<paso>.yaml`, NO está
// en la lista porque no vive bajo el directorio del step: no hay nada de lo que
// excluirlo.
var declarationFileNames = []string{"commands.yaml", "config.yaml"}

// DeclarationFileNames devuelve una copia de la lista, para quien tenga que
// afirmar en un test que coincide con lo que el repositorio lee.
func DeclarationFileNames() []string {
	names := make([]string, len(declarationFileNames))
	copy(names, declarationFileNames)
	return names
}

// OutputMaterial es un `outputs:` de `commands.yaml` reducido a lo que la regla
// necesita.
type OutputMaterial struct {
	Name  string
	Probe string
}

// InstructionMaterial es un comando declarado en `commands.yaml`, reducido a lo
// que la regla necesita.
//
// Es un tipo propio del paquete y no `command.Command` a propósito: la regla se
// especifica y se valida con vectores en memoria (SPEC-PIPELINE-v1.md §6), y
// para eso no puede depender del modelo de ejecución. Quien traduce es el
// consumidor —`internal/domain/step`—, y ese paso de traducción es también el
// sitio donde se ve, en el compilador, que un campo nuevo de `commands.yaml`
// hay que decidir si entra o no.
type InstructionMaterial struct {
	Name      string
	Cmd       string
	Workdir   string
	Show      bool
	Templates []string
	Outputs   []OutputMaterial
}

// VariableMaterial es una variable DECLARADA en
// `variables/<ambiente>/<paso>.yaml`, reducida a lo que la regla necesita.
//
// `Declaration` es la forma canónica de la declaración —el literal
// entrecomillado, o la terna `resolve`/`from`/`key`/`scope`—, NUNCA el valor
// resuelto. Es el cambio de material que la spec 27 §5.2 hace y con el que
// cierra dos defectos de signo contrario a la vez: las salidas de una corrida
// dejan de ser entradas de la siguiente, y un digest sin sal de los valores de
// configuración deja de salir de la organización en cada `step_finished`
// (spec 20 §8).
//
// «Declarada y vacía» y «no declarada» son estados DISTINTOS y la regla los
// separa: el material serializa `Q(Declaration)`, y la declaración de un literal
// vacío es `""`, no ausencia (spec 03 §5.3).
type VariableMaterial struct {
	Name        string
	Declaration string
}

// StepDeclarationMaterial es TODO lo que un step declara sobre sí mismo, menos
// el árbol de su directorio —que llega por el puerto `TreeSource`, porque la
// regla lo recorre con la misma `Compute` que el árbol del proyecto—.
//
// Es un struct con campos exportados y exhaustivo a propósito, por lo mismo que
// lo era `cache.Material`: añadir una fuente al material tiene que ser un cambio
// de tipo, visible en el compilador, y no un olvido.
//
// Los tres campos pueden ser legítimamente su valor cero, y ninguno es «material
// incompleto»:
//
//   - `Scope` y `Rules` vacíos son un step SIN `config.yaml` — que se ejecuta
//     siempre y no persiste registro (spec 13 §5.3);
//   - `Commands` vacío es un step `skipped{no_commands}` (spec 04 §5.3);
//   - `Variables` vacío es un step que no declara variables, y el archivo
//     ausente es válido desde siempre (spec 24 §5.3).
type StepDeclarationMaterial struct {
	// Scope es el `scope:` DECLARADO en `config.yaml`, no la dirección que de él
	// se deriva (spec 27 §5.2bis). Lo primero es contenido; lo segundo es la
	// clave de estado, y no entra en ninguna huella.
	Scope string

	// Rules es la forma canónica del conjunto de reglas declarado en
	// `config.yaml` (spec 15). Entra el VALOR DE DOMINIO y no el texto del
	// archivo: `- state_changed` y `- state_changed: [pipeline, project]`
	// declaran lo mismo y producen la misma cadena.
	Rules string

	// Commands son los comandos declarados, EN SU ORDEN.
	Commands []InstructionMaterial

	// Variables son las declaraciones de variables del step para el ambiente en
	// ejecución.
	Variables []VariableMaterial
}

// ComputeStepDeclaration calcula la huella `pipe-v1` de lo que un step DECLARA
// hacer: sus tres archivos de declaración, normalizados, más el árbol crudo del
// resto de su directorio.
//
// La regla completa y normativa está en SPEC-PIPELINE-v1.md. En resumen: una
// cabecera, el ámbito y las reglas, los comandos en el orden declarado, las
// declaraciones de variables ordenadas, y la huella del árbol del directorio del
// step por su forma canónica COMPLETA; líneas unidas por "\n" sin salto final, y
// sha256 del resultado.
//
// # El árbol entra por la MISMA `Compute` que el del proyecto
//
// No hay una variante «para el pipelinecode» y no debe haberla (spec 27 §5.2',
// LSP): dos raíces, una regla. Lo único que se interpone es la exclusión de los
// archivos que ya entran normalizados — sin ella, un comentario en
// `commands.yaml` re-ejecutaría el step por la puerta de atrás, y el reparto de
// §5.3 («normalizado lo que el motor entiende, crudo lo que no») dejaría de ser
// cierto.
func ComputeStepDeclaration(m StepDeclarationMaterial, tree TreeSource) (Fingerprint, error) {
	if tree == nil {
		return Fingerprint{}, errors.New(
			"fingerprint: la declaración del step no trae el árbol de su directorio")
	}

	treeFingerprint, err := Compute(excludingSource{src: tree, excluded: declarationFileNames})
	if err != nil {
		return Fingerprint{}, fmt.Errorf("fingerprint: árbol del directorio del step: %w", err)
	}

	lines := make([]string, 0, 6+len(m.Commands)+len(m.Variables))
	lines = append(lines,
		declarationHeader,
		strconv.Quote(m.Scope),
		strconv.Quote(m.Rules),
		// La cardinalidad va delante de cada lista para que la regla se pueda
		// leer sin adivinar dónde termina un bloque y empieza el siguiente.
		strconv.Itoa(len(m.Commands)),
	)
	for _, c := range m.Commands {
		lines = append(lines, instructionEntry(c))
	}

	entries := make([]string, 0, len(m.Variables))
	for _, v := range m.Variables {
		entries = append(entries, variableEntry(v))
	}
	// Ordenar por la entrada completa, no por el nombre: es la misma disciplina
	// que la huella del árbol (SPEC-v1.md §3.3) y da un orden total aunque el
	// llamador entregue dos declaraciones con el mismo nombre, cosa que un mapa
	// no puede pero un slice sí.
	sort.Strings(entries)

	lines = append(lines, strconv.Itoa(len(entries)))
	lines = append(lines, entries...)

	// La forma canónica COMPLETA de la huella del árbol, con su prefijo: componer
	// sobre el hash pelado haría iguales un árbol identificado con `v1` y el mismo
	// identificado con `v2`, y con ello un salto de versión de la regla del árbol
	// dejaría de invalidar estas huellas.
	lines = append(lines, strconv.Quote(treeFingerprint.String()))

	sum := sha256.Sum256([]byte(strings.Join(lines, entrySeparator)))
	return newVersioned(DeclarationVersion, hex.EncodeToString(sum[:]))
}

// instructionEntry es la línea de un comando (SPEC-PIPELINE-v1.md §3.2).
//
// `Show` entra en el material (spec 10 §5.1bis) aunque no cambie qué se ejecuta:
// si no entrara, añadir `show: true` para depurar un comando no invalidaría el
// caché, el step se saltaría y no se imprimiría nada — una huella que ignora una
// edición deliberada del pipelinecode es indistinguible de una huella rota.
func instructionEntry(c InstructionMaterial) string {
	fields := make([]string, 0, 6+len(c.Templates)+2*len(c.Outputs))

	fields = append(fields,
		strconv.Quote(c.Name),
		strconv.Quote(c.Cmd),
		strconv.Quote(c.Workdir),
		strconv.FormatBool(c.Show),
	)

	// La cardinalidad va delante de cada lista: sin ella, dos comandos con
	// listas de distinto reparto podrían producir la misma cadena.
	fields = append(fields, strconv.Itoa(len(c.Templates)))
	for _, template := range c.Templates {
		fields = append(fields, strconv.Quote(template))
	}

	fields = append(fields, strconv.Itoa(len(c.Outputs)))
	for _, output := range c.Outputs {
		fields = append(fields, strconv.Quote(output.Name), strconv.Quote(output.Probe))
	}

	return strings.Join(fields, fieldSep)
}

// variableEntry es la línea de una declaración de variable
// (SPEC-PIPELINE-v1.md §3.3).
func variableEntry(v VariableMaterial) string {
	return strings.Join([]string{
		strconv.Quote(v.Name),
		strconv.Quote(v.Declaration),
	}, fieldSep)
}

// excludingSource es el puerto `TreeSource` con unos cuantos archivos de la RAÍZ
// escondidos.
//
// Vive en el dominio y no en infraestructura porque la exclusión es parte de la
// REGLA: quien reimplemente `pipe-v1` tiene que aplicarla, y una implementación
// que la pusiera en su fuente de árbol produciría la misma huella por casualidad
// hasta el día que sirviera el árbol de otra forma.
//
// Sólo esconde las entradas de la raíz: un `terraform/commands.yaml` es un
// archivo arbitrario del step como cualquier otro, y el motor no lo interpreta.
type excludingSource struct {
	src      TreeSource
	excluded []string
}

func (s excludingSource) Walk(fn WalkFunc) error {
	return s.src.Walk(func(path string, isDir, isSymlink bool, mode fs.FileMode) error {
		if !isDir && s.hides(path) {
			return nil
		}
		return fn(path, isDir, isSymlink, mode)
	})
}

func (s excludingSource) Open(path string) (io.ReadCloser, error) { return s.src.Open(path) }

func (s excludingSource) hides(path string) bool {
	if strings.Contains(path, "/") {
		return false
	}
	for _, name := range s.excluded {
		if path == name {
			return true
		}
	}
	return false
}
