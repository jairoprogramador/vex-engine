package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

var errAyuda = errors.New("ayuda pedida")

type opciones struct {
	rutas   rutas
	entrada string
}

func leerOpciones(op operacion, args []string, errores io.Writer) (opciones, error) {
	var o opciones
	fs := flag.NewFlagSet(op.nombre, flag.ContinueOnError)
	fs.SetOutput(errores)
	fs.StringVar(&o.rutas.almacen, "almacen", os.Getenv(nombreAlmacen), "almacén del historial")
	fs.StringVar(&o.rutas.espacio, "espacio", os.Getenv(nombreEspacio), "espacio de trabajo de los ambientes")
	fs.StringVar(&o.rutas.material, "material", os.Getenv(nombreMaterial), "material de las fuentes")
	fs.StringVar(&o.entrada, "entrada", entradaEstandar, "petición en JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return o, errAyuda
		}
		return o, err
	}
	if fs.NArg() > 0 {
		return o, fmt.Errorf("sobran argumentos: %v", fs.Args())
	}
	if o.rutas.almacen == "" {
		return o, fmt.Errorf("falta --almacen (o $%s): dónde está el historial", nombreAlmacen)
	}
	if op.usaEspacio && o.rutas.espacio == "" {
		return o, fmt.Errorf("falta --espacio (o $%s): dónde trabajan los pasos de cada ambiente", nombreEspacio)
	}
	if o.rutas.material == "" {
		o.rutas.material = materialPorDefecto()
	}
	return o, nil
}

func materialPorDefecto() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "vex", "material")
}

func leerPeticion(fichero string, entrada io.Reader) ([]byte, error) {
	if fichero == entradaEstandar {
		datos, err := io.ReadAll(entrada)
		if err != nil {
			return nil, fmt.Errorf("leer la petición de la entrada estándar: %w", err)
		}
		return datos, nil
	}
	datos, err := os.ReadFile(fichero)
	if err != nil {
		return nil, fmt.Errorf("leer la petición: %w", err)
	}
	return datos, nil
}

func escribirRespuesta(w io.Writer, respuesta any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(respuesta)
}

// codigoDeLaRespuesta traduce cómo terminó un intento en el código de salida: la operación se atendió bien,
// pero el pipeline pudo fallar o cancelarse, y quien invoca desde un script lo necesita sin leer el JSON.
func codigoDeLaRespuesta(respuesta any) int {
	r, ok := respuesta.(ejecucionpublicado.Resultado)
	switch {
	case !ok || r.Estado == estadoExitoso:
		return salidaBien
	case r.Estado == estadoCancelado:
		return salidaCancelado
	default:
		return salidaFallo
	}
}

// salidaEnVivo entrega lo que imprimen los comandos a un escritor, tal cual y sin guardarlo (DEC-12.5). Un
// comando escribe su salida y su error desde hilos distintos, así que serializa.
type salidaEnVivo struct {
	mu sync.Mutex
	w  io.Writer
}

var _ ejecucionpublicado.Salida = (*salidaEnVivo)(nil)

func (s *salidaEnVivo) Escribir(_ string, datos []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.w.Write(datos)
	return err
}
