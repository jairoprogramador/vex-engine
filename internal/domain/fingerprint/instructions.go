package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// InstructionsVersion identifica la regla de la huella de instrucciones.
//
// Es un token DISTINTO del de la huella del árbol a propósito. Las dos son
// `Fingerprint` y las dos entran en el mismo material de `cache.Material`, pero
// son reglas distintas con historias distintas: si mañana la del árbol salta a
// `v2` y ésta no, los dos prefijos tienen que poder moverse por separado. Con un
// token compartido, «versión de la regla» dejaría de significar nada.
const InstructionsVersion = "inst-v1"

// fieldSep separa los campos dentro de la entrada de un comando.
//
// Es U+001E (RS) y no `:` como en la huella del árbol porque aquí todos los
// valores textuales viajan entre comillas de `strconv.Quote`, que escapa
// cualquier carácter de control: ni el separador ni el salto de línea pueden
// aparecer dentro de un campo. Esa es la razón por la que esta regla NO hereda
// la ambigüedad teórica del separador que la huella del árbol se dejó congelada
// (SPEC-v1.md §6).
const fieldSep = "\x1e"

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
// especifica y se valida con vectores en memoria (SPEC-INSTRUCTIONS-v1.md §6),
// y para eso no puede depender del modelo de ejecución. Quien traduce es el
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

// ComputeInstructions calcula la huella de una lista de comandos declarados.
//
// La regla completa y normativa está en SPEC-INSTRUCTIONS-v1.md. En resumen:
// una entrada por comando, en el ORDEN DECLARADO —el orden es semántico, no
// cosmético—, campos entrecomillados y separados por U+001E, entradas unidas por
// "\n" sin salto final, y sha256 del resultado.
//
// `Show` entra en el material (spec 10 §5.1bis) aunque no cambie qué se ejecuta:
// si no entrara, añadir `show: true` para depurar un comando no invalidaría el
// caché, el step se saltaría y no se imprimiría nada — un caché que ignora una
// edición deliberada del pipelinecode es indistinguible de un caché roto.
func ComputeInstructions(commands []InstructionMaterial) (Fingerprint, error) {
	entries := make([]string, 0, len(commands))
	for _, c := range commands {
		entries = append(entries, instructionEntry(c))
	}

	sum := sha256.Sum256([]byte(strings.Join(entries, entrySeparator)))
	return newVersioned(InstructionsVersion, hex.EncodeToString(sum[:]))
}

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
