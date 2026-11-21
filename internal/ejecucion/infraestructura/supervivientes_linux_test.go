//go:build linux

package infraestructura_test

import (
	"context"
	"fmt"
	"os"
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

// permisoParaMatarlosATodos es la variable con que se dice, a propósito, que estas pruebas pueden matar todos los
// procesos de su espacio. Sin ella no corren ni siendo el proceso 1.
const permisoParaMatarlosATodos = "VEX_PRUEBA_PROCESO_1"

// soloComoProceso1 salta la prueba si no corre como proceso 1 de un contenedor desechable. Las pruebas que lo
// llaman hacen kill(-1) de verdad: nunca en la máquina de nadie. Cómo correrlas (desde la raíz del motor):
//
//	docker run --rm -e VEX_PRUEBA_PROCESO_1=1 -v "$PWD":/src -w /src golang:1.27 sh -c \
//	  'go test -c -o /tmp/infra.test ./internal/ejecucion/infraestructura && cd internal/ejecucion/infraestructura &&
//	   exec /tmp/infra.test -test.run Proceso1 -test.v'
//
// El exec es lo que hace que el binario de pruebas sea el proceso 1, como vexd en su contenedor.
func soloComoProceso1(t *testing.T) {
	t.Helper()
	if os.Getpid() != 1 || os.Getenv(permisoParaMatarlosATodos) != "1" {
		t.Skipf("solo como proceso 1 de un contenedor desechable y con %s=1", permisoParaMatarlosATodos)
	}
}

// existe dice si el proceso sigue en la tabla de procesos, vivo o zombi: sin ps, que también moriría.
func existe(pid int) bool { return syscall.Kill(pid, 0) == nil }

// estaVivo dice si el proceso existe y no es un zombi: el tercer campo de /proc/<pid>/stat es su estado.
func estaVivo(pid int) bool {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	trasElNombre := string(stat[strings.LastIndexByte(string(stat), ')')+1:])
	return !strings.HasPrefix(strings.TrimSpace(trasElNombre), "Z")
}

// ejecutarQueDejaUnProceso corre un comando que deja un proceso vivo de la forma que diga la plantilla (con un
// %s para el fichero donde apunta su pid) y devuelve ese pid, ya con el comando terminado.
func ejecutarQueDejaUnProceso(t *testing.T, plantilla string) int {
	t.Helper()
	apunte := filepath.Join(t.TempDir(), "pid")
	linea := fmt.Sprintf(plantilla, apunte)
	resultado, err := infraestructura.NuevosComandos().Ejecutar(
		context.Background(), t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), dominio.Entorno{},
		dominio.NuevaSalidaDeComando())
	require.NoError(t, err)
	require.True(t, resultado.Exitoso)
	var pid int
	require.Eventually(t, func() bool {
		contenido, err := os.ReadFile(apunte)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(contenido)))
		return err == nil && pid > 1
	}, 5*time.Second, 10*time.Millisecond, "el proceso no llegó a apuntar su pid")
	return pid
}

// El ataque que esto cierra: un comando de un paso deja un proceso en su propia sesión, que no muere con el
// grupo del comando, y se queda esperando a lo que reciba el paso siguiente.
func TestProceso1_UnProcesoQueEscapoConSetsidNoLlegaAlPasoSiguiente(t *testing.T) {
	soloComoProceso1(t)
	escapado := ejecutarQueDejaUnProceso(t, "setsid sh -c 'echo $$ > %s; exec sleep 300' >/dev/null 2>&1 </dev/null &\nsleep 0.3")
	require.True(t, estaVivo(escapado), "sin esto, el proceso sobrevive a su comando: es lo que hay que cerrar")

	require.NoError(t, infraestructura.NuevosSupervivientes().Acabar(context.Background()))

	require.False(t, existe(escapado), "ni vivo ni zombi: se le mató y se le recogió")
}

func TestProceso1_AcabaConTodoUnArbolYNoDejaZombis(t *testing.T) {
	soloComoProceso1(t)
	// Un demonio de doble fork con hijos propios, y otro proceso suelto: ninguno es hijo directo del motor.
	demonio := ejecutarQueDejaUnProceso(t,
		"setsid sh -c 'echo $$ > %s; sleep 300 & sleep 300 & wait' >/dev/null 2>&1 </dev/null &\nsleep 0.3")
	suelto := ejecutarQueDejaUnProceso(t, "setsid sh -c 'echo $$ > %s; trap \"\" TERM HUP INT; exec sleep 300' >/dev/null 2>&1 </dev/null &\nsleep 0.3")
	require.True(t, estaVivo(demonio))
	require.True(t, estaVivo(suelto))

	require.NoError(t, infraestructura.NuevosSupervivientes().Acabar(context.Background()))

	require.False(t, existe(demonio))
	require.False(t, existe(suelto), "ignorar SIGTERM no salva de SIGKILL")
	var estado syscall.WaitStatus
	_, err := syscall.Wait4(-1, &estado, syscall.WNOHANG, nil)
	require.ErrorIs(t, err, syscall.ECHILD, "no queda ningún hijo por recoger")
	procesos, err := filepath.Glob("/proc/[0-9]*")
	require.NoError(t, err)
	require.Equal(t, []string{"/proc/1"}, procesos, "en el contenedor solo queda el motor")
}

func TestProceso1_SinNadieMasNoFallaYSePuedeSeguirEjecutando(t *testing.T) {
	soloComoProceso1(t)
	supervivientes := infraestructura.NuevosSupervivientes()

	require.NoError(t, supervivientes.Acabar(context.Background()), "sin procesos que matar no es un error")
	require.NoError(t, supervivientes.Acabar(context.Background()))

	// Después de matarlos a todos, el motor sigue pudiendo ejecutar el comando del paso siguiente.
	salida := dominio.NuevaSalidaDeComando()
	linea := "echo sigo-vivo"
	resultado, err := infraestructura.NuevosComandos().Ejecutar(
		context.Background(), t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), dominio.Entorno{}, salida)
	require.NoError(t, err)
	require.True(t, resultado.Exitoso)
	require.Contains(t, salida.Texto(), "sigo-vivo")
}
