// Command vexd es la raíz de composición del motor: conecta los contextos y atiende, con el borde, una
// operación del lenguaje publicado por invocación (docs/modelo/lenguaje-publicado.md).
//
// Habla JSON-RPC 2.0 por la entrada y la salida estándar, un mensaje JSON por línea
// (docs/rediseno/RD-13-protocolo.md): lee una petición, responde con una línea y termina. No tiene subcomandos
// ni opciones. Lo que necesita del entorno lo lee de variables:
//
//	VEX_ALMACEN   almacén del historial; el directorio tiene que existir
//	VEX_ESPACIO   espacio de trabajo de los ambientes, para intentar y rollback
//	VEX_MATERIAL  donde se pone el material de las fuentes; una copia desechable
//
// La salida estándar es solo protocolo. Lo que imprimen los comandos de los pasos no se muestra: se guarda en el
// historial y se consulta con logs. En la salida de error va solo la causa de un error interno.
//
// Códigos de salida: 0 bien · 1 la operación falló, o el intento terminó fallido · 2 la petición o la
// configuración son inválidas · 130 cancelado por señal o por la notificación cancelar. Cancelar le pide a cada
// comando que termine y, pasado el plazo de gracia, acaba con él y con lo que lanzó: más señales no matan a vexd.
package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
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
	estadoFallido    = "fallido"
	estadoCancelado  = "cancelado"
	marcaDeOperacion = "vexd"
)

func main() {
	// La primera señal cancela. Las siguientes no matan a vexd: cada comando corre en su propio grupo de procesos,
	// así que morir de golpe dejaría huérfano lo que lanzó y el intento sin cerrar. Cancelar tiene un plazo (los
	// comandos que no terminan con SIGTERM mueren al acabarse el de gracia), así que vexd siempre acaba solo.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(ejecutar(ctx, rutasDelEntorno(os.Getenv), os.Stdin, os.Stdout, os.Stderr))
}

// rutasDelEntorno lee la configuración del proceso. Es lo único que lee el entorno: ejecutar recibe las rutas ya
// resueltas. Que falte una obligatoria se dice cuando se sabe qué operación se pide, no aquí.
func rutasDelEntorno(getenv func(string) string) rutas {
	r := rutas{
		almacen:  getenv(nombreAlmacen),
		espacio:  getenv(nombreEspacio),
		material: getenv(nombreMaterial),
	}
	if r.material == "" {
		r.material = materialPorDefecto()
	}
	return r
}

// materialPorDefecto es un directorio temporal: el material es una copia que se puede borrar sin que cambie
// ninguna decisión, y en un contenedor efímero muere con él.
func materialPorDefecto() string {
	return filepath.Join(os.TempDir(), "vex", "material")
}
