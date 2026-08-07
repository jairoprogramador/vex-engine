package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

// VariablesVersion identifica la regla de la huella de variables. Token propio
// por la misma razón que InstructionsVersion.
const VariablesVersion = "vars-v1"

// VariableMaterial es una variable del mapa acumulado reducida a lo que la regla
// necesita.
//
// «Declarada y vacía» y «no declarada» son estados DISTINTOS y la regla los
// separa: el material serializa `strconv.Quote(Value)`, y `Quote("")` es `""`,
// no ausencia (spec 03 §5.3).
type VariableMaterial struct {
	Name  string
	Value string

	// Shared es CONSTANTE `false` desde la spec 13: el ámbito dejó de ser un
	// atributo de la variable y pasó a ser del step. El campo sigue aquí porque
	// la regla está congelada —§3.2 exige tres campos por entrada— y quitarlo
	// sería un `vars-v2` con su propia especificación, a cambio de nada: el valor
	// `true` no era alcanzable en ninguna ejecución real, así que ninguna huella
	// ya emitida se mueve. Muere con la regla en la spec 27.
	Shared bool
}

// ComputeVariables calcula la huella de un conjunto de variables.
//
// La regla completa y normativa está en SPEC-VARIABLES-v1.md. En resumen: una
// entrada por variable, ordenadas lexicográficamente, unidas por "\n" sin salto
// final, y sha256 del resultado.
//
// La regla es PURA sobre lo que se le da: no sabe qué variables son volátiles.
// El filtro previo es normativo igualmente y está en §3.1 de la especificación,
// aplicado por el consumidor —`step.NewCacheMaterial`— con la lista que declara
// `command.VolatileVarNames`. Se parte así porque los nombres volátiles son
// vocabulario del modelo de ejecución, no de la regla de huella; lo que la
// especificación exige es que la lista esté escrita, no dónde vive.
func ComputeVariables(variables []VariableMaterial) (Fingerprint, error) {
	entries := make([]string, 0, len(variables))
	for _, variable := range variables {
		entries = append(entries, variableEntry(variable))
	}

	// Ordenar por la entrada completa, no por el nombre: es la misma disciplina
	// que la huella del árbol (SPEC-v1.md §3.3) y da un orden total aunque el
	// llamador entregue dos variables con el mismo nombre, cosa que un mapa no
	// puede pero un slice sí.
	sort.Strings(entries)

	sum := sha256.Sum256([]byte(strings.Join(entries, entrySeparator)))
	return newVersioned(VariablesVersion, hex.EncodeToString(sum[:]))
}

func variableEntry(v VariableMaterial) string {
	return strings.Join([]string{
		strconv.Quote(v.Name),
		strconv.Quote(v.Value),
		strconv.FormatBool(v.Shared),
	}, fieldSep)
}
