package dominio

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"slices"
	"strconv"
	"strings"
)

// CalcularHashDeInstrucciones es el único punto donde se calcula el hash de instrucciones de un paso: sus
// comandos y el material de su directorio, y nunca su configuración — reglas, edad máxima, ámbito (DEC-08.7).
// Es un cálculo puro: solo toma lo que ComandoDeclarado y FicheroDeclarado ya exponen, sin leer disco — leerlos
// es cosa de la infraestructura que traduce lo que Definición publica (ver ejecucion/infraestructura.Pipelines).
//
// Determinista sin depender del orden de entrada de material, que se ordena por ruta antes de entrar — igual
// que suministro/infraestructura.HashDeContenido ordena por ruta el árbol de un directorio. Los comandos SÍ
// dependen de su propio orden de declaración: cambiar el orden en que corren es un cambio real de
// instrucciones, así que ese no se reordena.
const prefijoHashDeInstrucciones = "instrucciones-v1:"

func CalcularHashDeInstrucciones(comandos []ComandoDeclarado, material []FicheroDeclarado) HashDeInstrucciones {
	total := sha256.New()

	for _, c := range comandos {
		io.WriteString(total, "comando\x00")
		io.WriteString(total, c.Nombre()+"\x00")
		io.WriteString(total, c.Linea()+"\x00")
		io.WriteString(total, c.Directorio()+"\x00")
		for _, p := range c.Plantillas() {
			io.WriteString(total, "plantilla:"+p+"\x00")
		}
		for _, s := range c.Salidas() {
			io.WriteString(total, "salida:"+s.Nombre()+"\x00"+s.Expresion()+"\x00"+strconv.FormatBool(s.Compartida())+"\x00")
		}
		for _, a := range c.Aserciones() {
			io.WriteString(total, "asercion:"+a.Expresion()+"\x00")
		}
	}

	ordenado := slices.Clone(material)
	slices.SortFunc(ordenado, func(a, b FicheroDeclarado) int { return strings.Compare(a.Ruta(), b.Ruta()) })
	for _, f := range ordenado {
		io.WriteString(total, "fichero\x00")
		io.WriteString(total, f.Ruta()+"\x00")
		io.WriteString(total, f.Contenido()+"\x00")
		io.WriteString(total, strconv.FormatBool(f.Ejecutable())+"\x00")
		io.WriteString(total, f.Enlace()+"\x00")
		io.WriteString(total, strconv.FormatBool(f.Plantilla())+"\x00")
	}

	return HashDeInstrucciones{valor: prefijoHashDeInstrucciones + hex.EncodeToString(total.Sum(nil))}
}
