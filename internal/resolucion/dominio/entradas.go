package dominio

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// HashDeLasEntradas resume, en un solo hash, las variables que un paso consume: si cambia, alguna cambió de
// nombre o de valor. Quien lo pide ya dejó fuera las que no son una entrada (las generadas por el motor, que
// describen la corrida y no el pipeline). Es determinista sin depender del orden en que se agregaron, y se
// apoya en CalcularHashDeVariable: aquí el valor en claro nunca se compone dos veces.
func HashDeLasEntradas(entradas []VariableEfectiva) HashDeEntradas {
	ordenadas := make([]VariableEfectiva, len(entradas))
	copy(ordenadas, entradas)
	sort.Slice(ordenadas, func(i, j int) bool { return ordenadas[i].Nombre() < ordenadas[j].Nombre() })

	total := sha256.New()
	for _, e := range ordenadas {
		total.Write([]byte(e.Nombre() + "\x00" + CalcularHashDeVariable(e.Valor()).String() + "\x00"))
	}
	return HashDeEntradas{valor: prefijoHashDeEntradas + hex.EncodeToString(total.Sum(nil))}
}

// HashDeEntradas es el resumen de las variables que un paso consume. Se calcula solo con HashDeLasEntradas.
type HashDeEntradas struct{ valor string }

const prefijoHashDeEntradas = "entradas-v2:"

func (h HashDeEntradas) String() string { return h.valor }
