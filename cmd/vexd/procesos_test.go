//go:build !windows

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Cada contenedor es un proceso vexd: comparten el almacén y el espacio de trabajo, y no se hablan entre sí. Estas
// pruebas lanzan procesos de verdad (el binario de las pruebas, ver senales_test.go) porque lo que se prueba es la
// exclusión entre procesos, que con goroutines de un mismo proceso no es lo mismo.

type procesoVexd struct {
	cmd             *exec.Cmd
	salida, errores bytes.Buffer
	terminado       chan error
}

func lanzarVexd(t *testing.T, r rutas, metodo, params string) *procesoVexd {
	t.Helper()
	p := &procesoVexd{cmd: exec.Command(os.Args[0]), terminado: make(chan error, 1)}
	p.cmd.Env = append(os.Environ(), variableComoProceso+"=1",
		nombreAlmacen+"="+r.almacen, nombreEspacio+"="+r.espacio, nombreMaterial+"="+r.material)
	p.cmd.Stdin = strings.NewReader(peticion(metodo, params) + "\n")
	p.cmd.Stdout, p.cmd.Stderr = &p.salida, &p.errores
	require.NoError(t, p.cmd.Start())
	go func() { p.terminado <- p.cmd.Wait() }()
	t.Cleanup(func() { _ = p.cmd.Process.Kill() })
	return p
}

// esperar devuelve cómo terminó el proceso.
func (p *procesoVexd) esperar(t *testing.T) invocacion {
	t.Helper()
	select {
	case err := <-p.terminado:
		codigo := 0
		var salio *exec.ExitError
		if errors.As(err, &salio) {
			codigo = salio.ExitCode()
		} else {
			require.NoError(t, err)
		}
		return invocacion{codigo, p.salida.String(), p.errores.String()}
	case <-time.After(60 * time.Second):
		t.Fatal("el proceso no terminó")
		return invocacion{}
	}
}

func TestProcesos_DosVexdEnElMismoAmbienteUnoGanaYElOtroNoTocaSuEspacio(t *testing.T) {
	carpeta := t.TempDir()
	listo, ruta := filepath.Join(carpeta, "listo"), filepath.Join(carpeta, "ruta")
	e := nuevoEntornoConPipeline(t, func(dir string) {
		escribir(t, dir, "steps/05-deploy/commands.yaml", "- name: comando-deploy-lento\n  description: escribe en su espacio y tarda\n"+
			"  cmd: pwd > "+ruta+"; echo vivo > marca-del-intento; touch "+listo+"; sleep 3\n")
	})
	primero := lanzarVexd(t, e.rutas, "intentar", e.intento())
	require.Eventually(t, func() bool { _, err := os.Stat(listo); return err == nil },
		60*time.Second, 20*time.Millisecond, "el primer intento no llegó a su comando lento")
	contenido, err := os.ReadFile(ruta)
	require.NoError(t, err)
	marca := filepath.Join(strings.TrimSpace(string(contenido)), "marca-del-intento")
	require.FileExists(t, marca)

	segundo := lanzarVexd(t, e.rutas, "intentar", e.intento()).esperar(t)

	require.Equal(t, salidaFallo, segundo.codigo, segundo.salida)
	rechazo := segundo.error(t)
	require.Equal(t, codigoAmbienteOcupado, rechazo.Error.Code)
	require.NotEmpty(t, rechazo.Error.Data["intento"], "dice qué intento ocupa el ambiente")
	require.FileExists(t, marca, "el segundo proceso no borró el espacio de trabajo del primero, que sigue corriendo")

	final := primero.esperar(t)
	require.Equal(t, salidaBien, final.codigo, final.errores)
	var resultado struct {
		Intento, Estado string
	}
	final.resultado(t, &resultado)
	require.Equal(t, "exitoso", resultado.Estado)
	require.Equal(t, resultado.Intento, rechazo.Error.Data["intento"], "el que ocupaba el ambiente era el primero")
}

func TestProcesos_AmbientesDistintosCorrenEnParaleloSinPisarse(t *testing.T) {
	e := nuevoEntorno(t)
	porAmbiente := func(ambiente string) string {
		return strings.Replace(e.intento(), `"Ambiente":"prod"`, `"Ambiente":"`+ambiente+`"`, 1)
	}

	a := lanzarVexd(t, e.rutas, "intentar", porAmbiente("sand"))
	b := lanzarVexd(t, e.rutas, "intentar", porAmbiente("stag"))
	c := lanzarVexd(t, e.rutas, "intentar", porAmbiente("prod"))

	for nombre, p := range map[string]*procesoVexd{"sand": a, "stag": b, "prod": c} {
		r := p.esperar(t)
		require.Equal(t, salidaBien, r.codigo, "%s: %s %s", nombre, r.salida, r.errores)
	}
	var lista []map[string]any
	r := invocar(t, e.rutas, "despliegues", `{"Version":"1","Ambiente":"prod"}`)
	r.resultado(t, &lista)
	require.Len(t, lista, 1)
}
