//go:build !windows

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
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

// Un contenedor que muere sin cerrar su intento (kill -9, falta de memoria, se apaga la máquina) no puede dejar
// el ambiente bloqueado: el siguiente intento que lo encuentre ocupado y sin latidos lo recupera.
func TestProcesos_UnVexdMuertoConKillNoBloqueaElAmbienteParaSiempre(t *testing.T) {
	if testing.Short() {
		t.Skip("espera la ventana de vida real del motor")
	}
	carpeta := t.TempDir()
	listo, pidDelComando := filepath.Join(carpeta, "listo"), filepath.Join(carpeta, "pid")
	e := nuevoEntornoConPipeline(t, func(dir string) {
		// El primer intento se queda dormido; los siguientes encuentran la marca y terminan enseguida. exec deja
		// al propio sleep con el pid guardado: matar a vexd no lo mata, y la prueba no debe dejarlo suelto.
		escribir(t, dir, "steps/05-deploy/commands.yaml", "- name: comando-deploy-lento\n  description: se duerme la primera vez\n"+
			"  cmd: if [ -e "+listo+" ]; then true; else echo $$ > "+pidDelComando+"; touch "+listo+"; exec sleep 120; fi\n")
	})
	t.Cleanup(func() {
		if datos, err := os.ReadFile(pidDelComando); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(datos))); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	primero := lanzarVexd(t, e.rutas, "intentar", e.intento())
	require.Eventually(t, func() bool { _, err := os.Stat(listo); return err == nil },
		60*time.Second, 20*time.Millisecond, "el primer intento no llegó a su comando lento")
	require.NoError(t, primero.cmd.Process.Kill(), "kill -9: ni cierra el intento ni limpia nada")
	primero.esperar(t)

	// Acaba de morir: su último latido es reciente, así que el ambiente se ve ocupado enseguida y sin esperar.
	inmediato := lanzarVexd(t, e.rutas, "intentar", e.intento()).esperar(t)
	require.Equal(t, salidaFallo, inmediato.codigo, inmediato.salida)
	require.Equal(t, codigoAmbienteOcupado, inmediato.error(t).Error.Code)

	// Pasada la ventana de vida sin un solo latido más, el siguiente lo da por muerto, lo cierra y sigue.
	time.Sleep(historialpublicado.VentanaDeVida)
	siguiente := lanzarVexd(t, e.rutas, "intentar", e.intento()).esperar(t)
	require.Equal(t, salidaBien, siguiente.codigo, siguiente.salida+siguiente.errores)
	var resultado struct{ Intento, Estado string }
	siguiente.resultado(t, &resultado)
	require.Equal(t, "exitoso", resultado.Estado)

	var intentos []struct{ Id, Estado, Causa string }
	invocar(t, e.rutas, "intentos", `{"Version":"1","Ambiente":"prod"}`).resultado(t, &intentos)
	require.Len(t, intentos, 2)
	require.Equal(t, "fallido", intentos[0].Estado, "el que murió cuenta como un error")
	require.Equal(t, "interrumpido", intentos[0].Causa, "y dice por qué")
	require.Equal(t, resultado.Intento, intentos[1].Id)
	require.Equal(t, "exitoso", intentos[1].Estado)
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
