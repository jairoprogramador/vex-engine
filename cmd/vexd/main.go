// Command vexd es la raíz de composición del motor: conecta los contextos y atiende, con el borde, una
// operación del lenguaje publicado por invocación (docs/modelo/lenguaje-publicado.md).
//
// Uso: vexd <operación> --almacen <dir> [--espacio <dir>] [--material <dir>] [--entrada <fichero>]
//
// La petición es un JSON (por --entrada o por la entrada estándar) con los campos del lenguaje publicado. La
// respuesta es un JSON en la salida estándar. Lo que imprimen los comandos de los pasos va, en vivo y tal
// cual, a la salida de error (DEC-12.5). Un error se explica en la salida de error.
//
// Códigos de salida: 0 bien · 1 la operación falló, o el intento terminó fallido · 2 la invocación o la
// petición son inválidas · 130 cancelado por señal.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/jairoprogramador/vex-engine/internal/borde"
)

// version se sobrescribe con `-ldflags "-X main.version=<tag>"` en el build.
var version = "dev"

const (
	salidaBien       = 0
	salidaFallo      = 1
	salidaInvalida   = 2
	salidaCancelado  = 130
	nombreAlmacen    = "VEX_ALMACEN"
	nombreEspacio    = "VEX_ESPACIO"
	nombreMaterial   = "VEX_MATERIAL"
	estadoExitoso    = "exitoso"
	estadoCancelado  = "cancelado"
	entradaEstandar  = "-"
	marcaDeOperacion = "motor"
)

func main() {
	// Una segunda señal mata el proceso: stop() devuelve el comportamiento por defecto tras la primera.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); stop() }()

	os.Exit(ejecutar(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// ejecutar es main sin el proceso: recibe todo lo que toca, para poder probarlo entero.
func ejecutar(ctx context.Context, args []string, entrada io.Reader, salida, errores io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		uso(errores)
		if len(args) == 0 {
			return salidaInvalida
		}
		return salidaBien
	}
	if args[0] == "version" {
		fmt.Fprintln(salida, version)
		return salidaBien
	}

	op, ok := buscar(args[0])
	if !ok {
		fmt.Fprintf(errores, "%s: operación desconocida %q\n\n", marcaDeOperacion, args[0])
		uso(errores)
		return salidaInvalida
	}
	opciones, err := leerOpciones(op, args[1:], errores)
	if err != nil {
		if errors.Is(err, errAyuda) {
			return salidaBien
		}
		fmt.Fprintf(errores, "%s %s: %v\n", marcaDeOperacion, op.nombre, err)
		return salidaInvalida
	}

	peticion, err := leerPeticion(opciones.entrada, entrada)
	if err != nil {
		fmt.Fprintf(errores, "%s %s: %v\n", marcaDeOperacion, op.nombre, err)
		return salidaInvalida
	}
	servicio, err := componer(opciones.rutas)
	if err != nil {
		fmt.Fprintf(errores, "%s %s: %v\n", marcaDeOperacion, op.nombre, err)
		return salidaInvalida
	}

	respuesta, err := op.atender(ctx, servicio, peticion, &salidaEnVivo{w: errores})
	if err != nil {
		fmt.Fprintf(errores, "%s %s: %v\n", marcaDeOperacion, op.nombre, err)
		return codigoDeError(err)
	}
	if err := escribirRespuesta(salida, respuesta); err != nil {
		fmt.Fprintf(errores, "%s %s: la respuesta: %v\n", marcaDeOperacion, op.nombre, err)
		return salidaFallo
	}
	return codigoDeLaRespuesta(respuesta)
}

func codigoDeError(err error) int {
	switch {
	case errors.Is(err, errEntrada), errors.Is(err, borde.ErrVersionNoSoportada):
		return salidaInvalida
	case errors.Is(err, context.Canceled):
		return salidaCancelado
	default:
		return salidaFallo
	}
}

func uso(w io.Writer) {
	nombres := make([]string, 0, len(operaciones))
	for _, o := range operaciones {
		nombres = append(nombres, o.nombre)
	}
	sort.Strings(nombres)
	fmt.Fprintf(w, `uso: %[1]s <operación> [opciones]

Operaciones: %[2]s, version

Opciones (cada operación acepta las que usa):
  --almacen <dir>    almacén del historial; tiene que existir (o $%[3]s)
  --espacio <dir>    espacio de trabajo de los ambientes, para intentar y rollback (o $%[4]s)
  --material <dir>   donde se pone el material de las fuentes; una copia desechable (o $%[5]s)
  --entrada <fichero> petición en JSON; sin ella, la entrada estándar

Respuesta: JSON en la salida estándar. Salida de los comandos: salida de error, en vivo.
Lenguaje publicado: docs/modelo/lenguaje-publicado.md
`, marcaDeOperacion, strings.Join(nombres, ", "), nombreAlmacen, nombreEspacio, nombreMaterial)
}
