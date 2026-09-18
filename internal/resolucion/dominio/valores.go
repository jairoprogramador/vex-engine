package dominio

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Origen dice de dónde salió una variable efectiva: de un literal del pipeline o de lo que produjo un
// comando. DEC-08.4 solo distingue estas dos — no hay un origen aparte para "la última vez que un paso no se
// re-ejecutó": esas variables entran como producidas, porque aportan estructuralmente lo mismo que si se
// hubieran producido ahora (ver NoReejecutado en la aplicación).
type Origen int

const (
	OrigenDeclarada Origen = iota + 1
	OrigenProducida
)

var nombresDeOrigen = map[Origen]string{
	OrigenDeclarada: "declarada",
	OrigenProducida: "producida",
}

func (o Origen) String() string {
	if nombre, ok := nombresDeOrigen[o]; ok {
		return nombre
	}
	return fmt.Sprintf("origen(%d)", int(o))
}

func (o Origen) valida() bool {
	_, ok := nombresDeOrigen[o]
	return ok
}

// Ambito es compartido o el de un ambiente concreto. Una variable pertenece a un ámbito, nunca a un paso.
type Ambito struct {
	compartido bool
	ambiente   string
}

func AmbitoDeAmbiente(ambiente string) (Ambito, error) {
	if ambiente == "" {
		return Ambito{}, invalido("un ámbito de ambiente no puede tener el nombre vacío")
	}
	return Ambito{ambiente: ambiente}, nil
}

func AmbitoCompartido() Ambito {
	return Ambito{compartido: true}
}

func (a Ambito) EsCompartido() bool { return a.compartido }

func (a Ambito) Ambiente() string { return a.ambiente }

func (a Ambito) String() string {
	if a.compartido {
		return "compartido"
	}
	return a.ambiente
}

// Ve: ¿esta variable, declarada en el ámbito a, se ve desde el ámbito que pregunta (desde)? Un ámbito de
// ambiente solo se ve desde sí mismo; el compartido se ve desde cualquiera (DEC-06.12).
func (a Ambito) Ve(desde Ambito) bool {
	return a == desde || a.EsCompartido()
}

// VariableEfectiva es una variable con su valor, su origen y su ámbito, lista para interpolar o comparar. El
// valor nunca sale de aquí salvo por Valor(): String() no lo incluye (IT-04 DEC-04.7 — el valor no circula
// hacia lo publicado).
type VariableEfectiva struct {
	nombre string
	valor  string
	origen Origen
	ambito Ambito
}

func NuevaVariableEfectiva(nombre, valor string, origen Origen, ambito Ambito) (VariableEfectiva, error) {
	if nombre == "" {
		return VariableEfectiva{}, invalido("una variable efectiva no puede tener el nombre vacío")
	}
	if !origen.valida() {
		return VariableEfectiva{}, invalido("%q: origen inválido", nombre)
	}
	if ambito == (Ambito{}) {
		return VariableEfectiva{}, invalido("%q: una variable efectiva necesita un ámbito", nombre)
	}
	return VariableEfectiva{nombre: nombre, valor: valor, origen: origen, ambito: ambito}, nil
}

func (v VariableEfectiva) Nombre() string { return v.nombre }

// Valor es el único getter del valor en claro. Quien lo obtiene decide adónde va: a interpolar, o a la
// relación reservada. Nunca a un tipo publicado.
func (v VariableEfectiva) Valor() string { return v.valor }

func (v VariableEfectiva) Origen() Origen { return v.origen }

func (v VariableEfectiva) Ambito() Ambito { return v.ambito }

func (v VariableEfectiva) String() string {
	return fmt.Sprintf("%s (%s, %s)", v.nombre, v.origen, v.ambito)
}

// HashDeVariable es el hash de variable: si cambió, cambia (DEC-08.5). Se calcula solo con
// CalcularHashDeVariable; este tipo solo envuelve y compara uno ya calculado o leído del historial.
type HashDeVariable struct{ valor string }

func NuevoHashDeVariable(valor string) (HashDeVariable, error) {
	if valor == "" {
		return HashDeVariable{}, invalido("un hash de variable no puede estar vacío")
	}
	return HashDeVariable{valor: valor}, nil
}

func (h HashDeVariable) String() string { return h.valor }

const prefijoHashDeVariable = "variable-v1:"

// CalcularHashDeVariable es el único punto donde un valor en claro se convierte en hash: sin clave ni sal
// (DEC-08.8), así el mismo valor da siempre el mismo hash sin importar el ambiente (DEC-08.6). No es una
// promesa de seguridad: un valor corto o predecible se puede fuerza-bruta a partir de su hash — aceptado y
// documentado, no un descuido.
func CalcularHashDeVariable(valor string) HashDeVariable {
	suma := sha256.Sum256([]byte(valor))
	return HashDeVariable{valor: prefijoHashDeVariable + hex.EncodeToString(suma[:])}
}
