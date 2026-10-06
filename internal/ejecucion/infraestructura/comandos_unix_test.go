//go:build !windows

package infraestructura_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
)

// Cancelar un comando tiene que acabar con todo lo que lanzó, no solo con el shell: un proceso nieto huérfano
// sigue escribiendo en el espacio de trabajo y, si conserva la tubería de salida, deja colgada la espera.

// ejecutarYCancelar corre la línea y, cuando el comando dice que está listo (crea el fichero), cancela. Devuelve
// lo que Ejecutar devolvió y cuánto tardó en volver desde la cancelación.
func ejecutarYCancelar(t *testing.T, comandos infraestructura.Comandos, linea string, listo string) (salida string, err error, tardo time.Duration) {
	t.Helper()
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	var recibido bytes.Buffer
	terminado := make(chan error, 1)
	go func() {
		_, err := comandos.Ejecutar(ctx, t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), dominio.Entorno{}, &recibido)
		terminado <- err
	}()

	require.Eventually(t, func() bool { _, err := os.Stat(listo); return err == nil },
		10*time.Second, 20*time.Millisecond, "el comando no llegó a correr")
	inicio := time.Now()
	cancelar()
	select {
	case err = <-terminado:
		return recibido.String(), err, time.Since(inicio)
	case <-time.After(10 * time.Second):
		t.Fatal("Ejecutar no volvió tras cancelar: el comando dejó colgada la espera")
		return "", nil, 0
	}
}

// sigueVivo dice si el proceso existe y no es un zombi pendiente de recoger.
func sigueVivo(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	estado, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	return err == nil && !strings.HasPrefix(strings.TrimSpace(string(estado)), "Z") && strings.TrimSpace(string(estado)) != ""
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

func eliminarAlTerminar(t *testing.T, pid int) {
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
}

func TestComandos_LaCancelacionMataTambienALosProcesosQueElComandoLanzo(t *testing.T) {
	carpeta := t.TempDir()
	listo, pidFichero := filepath.Join(carpeta, "listo"), filepath.Join(carpeta, "pid")
	linea := "sleep 60 & echo $! > " + pidFichero + "; touch " + listo + "; wait"

	_, err, tardo := ejecutarYCancelar(t, infraestructura.NuevosComandos(), linea, listo)

	require.Error(t, err)
	require.Less(t, tardo, 5*time.Second, "la cancelación no espera a que el nieto termine solo")
	pid := leerPid(t, pidFichero)
	eliminarAlTerminar(t, pid)
	require.Eventually(t, func() bool { return !sigueVivo(pid) }, 5*time.Second, 50*time.Millisecond,
		"el proceso nieto sobrevivió a la cancelación")
}

func TestComandos_LaCancelacionPideTerminarConSIGTERMAntesDeMatar(t *testing.T) {
	// Un comando que atrapa SIGTERM puede limpiar antes de irse (terraform suelta el bloqueo de su estado).
	listo := filepath.Join(t.TempDir(), "listo")
	linea := "trap 'echo limpieza-hecha; exit 0' TERM; touch " + listo + "; while :; do sleep 0.05; done"

	salida, err, tardo := ejecutarYCancelar(t, infraestructura.NuevosComandosConPlazoDeGracia(5*time.Second), linea, listo)

	require.Error(t, err, "se canceló: sigue siendo un error de este puerto")
	require.Contains(t, salida, "limpieza-hecha", "tuvo oportunidad de terminar bien")
	require.Less(t, tardo, 3*time.Second, "terminó por su cuenta, sin esperar el plazo")
}

func TestComandos_UnProcesoQueIgnoraSIGTERMMuereAlAcabarElPlazo(t *testing.T) {
	carpeta := t.TempDir()
	listo, pidFichero := filepath.Join(carpeta, "listo"), filepath.Join(carpeta, "pid")
	linea := "trap '' TERM; echo $$ > " + pidFichero + "; touch " + listo + "; while :; do sleep 0.05; done"

	_, err, tardo := ejecutarYCancelar(t, infraestructura.NuevosComandosConPlazoDeGracia(300*time.Millisecond), linea, listo)

	require.Error(t, err)
	require.GreaterOrEqual(t, tardo, 250*time.Millisecond, "se le dio el plazo antes de matarlo")
	require.Less(t, tardo, 5*time.Second)
	pid := leerPid(t, pidFichero)
	eliminarAlTerminar(t, pid)
	require.Eventually(t, func() bool { return !sigueVivo(pid) }, 5*time.Second, 50*time.Millisecond)
}

func TestComandos_UnComandoExitosoQueDejaUnProcesoEnSegundoPlanoNoCuelgaLaEspera(t *testing.T) {
	// Sin cancelar nada: el comando sale con 0 pero deja algo vivo con la tubería abierta. Antes Run esperaba a que
	// ese proceso terminara, sin límite; ahora se acota, y no es un fallo del comando.
	pidFichero := filepath.Join(t.TempDir(), "pid")
	linea := "sleep 60 & echo $! > " + pidFichero + "; echo listo"
	comandos := infraestructura.NuevosComandosConPlazoDeGracia(200 * time.Millisecond)

	inicio := time.Now()
	resultado, err := comandos.Ejecutar(context.Background(), t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), dominio.Entorno{}, &bytes.Buffer{})

	eliminarAlTerminar(t, leerPid(t, pidFichero))
	require.NoError(t, err, "salió con 0: lo que dejó en segundo plano no es un fallo suyo")
	require.True(t, resultado.Exitoso)
	require.Less(t, time.Since(inicio), 10*time.Second)
}
