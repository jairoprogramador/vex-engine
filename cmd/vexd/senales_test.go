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
)

// variableComoProceso hace que el binario de las pruebas se comporte como vexd: es la forma de probar las
// señales de un proceso de verdad sin compilar nada aparte.
const variableComoProceso = "VEXD_COMO_PROCESO"

func TestMain(m *testing.M) {
	if os.Getenv(variableComoProceso) == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

// sigueVivo dice si el proceso existe y no es un zombi pendiente de recoger.
func sigueVivo(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	estado, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	texto := strings.TrimSpace(string(estado))
	return err == nil && texto != "" && !strings.HasPrefix(texto, "Z")
}

func leerPid(t *testing.T, fichero string) int {
	t.Helper()
	require.Eventually(t, func() bool {
		contenido, err := os.ReadFile(fichero)
		return err == nil && strings.TrimSpace(string(contenido)) != ""
	}, 10*time.Second, 20*time.Millisecond)
	contenido, err := os.ReadFile(fichero)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(contenido)))
	require.NoError(t, err)
	return pid
}

func TestSenales_VariasSenalesNoDejanNadaHuerfanoYElIntentoSeCierraCancelado(t *testing.T) {
	carpeta := t.TempDir()
	listo, pidFichero := filepath.Join(carpeta, "listo"), filepath.Join(carpeta, "pid")
	e := nuevoEntornoConPipeline(t, func(dir string) {
		escribir(t, dir, "steps/05-deploy/commands.yaml",
			"- name: comando-deploy-lento\n  description: tarda y no hace caso a SIGTERM\n  cmd: trap '' TERM; sleep 60 & echo $! > "+pidFichero+"; touch "+listo+"; wait\n")
	})
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), variableComoProceso+"=1",
		nombreAlmacen+"="+e.rutas.almacen, nombreEspacio+"="+e.rutas.espacio, nombreMaterial+"="+e.rutas.material)
	cmd.Stdin = strings.NewReader(peticion("intentar", e.intento()) + "\n")
	var salida, errores bytes.Buffer
	cmd.Stdout, cmd.Stderr = &salida, &errores
	require.NoError(t, cmd.Start())
	terminado := make(chan error, 1)
	go func() { terminado <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	require.Eventually(t, func() bool { _, err := os.Stat(listo); return err == nil },
		20*time.Second, 20*time.Millisecond, "el comando lento no llegó a correr")
	pidDelNieto := leerPid(t, pidFichero)
	t.Cleanup(func() { _ = syscall.Kill(pidDelNieto, syscall.SIGKILL) })

	// Quien invoca insiste: Ctrl-C varias veces seguidas, mientras el comando, que ignora SIGTERM, aún no ha
	// muerto. Una segunda señal no debe matar a vexd con lo que lanzó todavía vivo.
	for range 3 {
		_ = cmd.Process.Signal(syscall.SIGTERM) // si vexd ya murió, las aserciones de abajo dicen cómo
		time.Sleep(200 * time.Millisecond)
	}

	var salio *exec.ExitError
	select {
	case err := <-terminado:
		require.True(t, errors.As(err, &salio), "terminó con un código de salida: %v", err)
		require.Equal(t, salidaCancelado, salio.ExitCode(), errores.String())
	case <-time.After(20 * time.Second):
		t.Fatal("vexd no terminó tras las señales")
	}

	r := invocacion{salio.ExitCode(), salida.String(), errores.String()}
	var resultado struct{ Estado string }
	r.resultado(t, &resultado)
	require.Equal(t, "cancelado", resultado.Estado, "el intento se cerró: el ambiente no queda ocupado")
	require.Eventually(t, func() bool { return !sigueVivo(pidDelNieto) }, 5*time.Second, 50*time.Millisecond,
		"lo que el comando lanzó sobrevivió a vexd")
}
