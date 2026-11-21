//go:build !windows

package infraestructura_test

import (
	"context"
	"fmt"
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
	recibido := dominio.NuevaSalidaDeComando()
	terminado := make(chan error, 1)
	go func() {
		_, err := comandos.Ejecutar(ctx, t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), dominio.Entorno{}, recibido)
		terminado <- err
	}()

	require.Eventually(t, func() bool { _, err := os.Stat(listo); return err == nil },
		10*time.Second, 20*time.Millisecond, "el comando no llegó a correr")
	inicio := time.Now()
	cancelar()
	select {
	case err = <-terminado:
		return recibido.Texto(), err, time.Since(inicio)
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
	resultado, err := comandos.Ejecutar(context.Background(), t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), dominio.Entorno{}, dominio.NuevaSalidaDeComando())

	require.NoError(t, err, "salió con 0: lo que dejó en segundo plano no es un fallo suyo")
	require.True(t, resultado.Exitoso)
	require.Less(t, time.Since(inicio), 10*time.Second)
	yaNoVive(t, leerPid(t, pidFichero), "y lo que dejó no sobrevive al comando")
}

// Lo que un comando lanza muere con él, también cuando termina por su cuenta (AJ-08): un proceso que un paso deja
// en segundo plano no puede seguir ahí cuando corre el siguiente, que puede traer otras credenciales.

// ejecutarSinCancelar corre la línea hasta que termina sola, con un plazo corto para que la espera a las tuberías
// que un proceso en segundo plano deja abiertas no alargue la prueba.
func ejecutarSinCancelar(t *testing.T, linea string) dominio.ResultadoDeUnComando {
	t.Helper()
	comandos := infraestructura.NuevosComandosConPlazoDeGracia(200 * time.Millisecond)
	resultado, err := comandos.Ejecutar(context.Background(), t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), dominio.Entorno{}, dominio.NuevaSalidaDeComando())
	require.NoError(t, err)
	return resultado
}

// yaNoVive espera, como mucho 2 s, a que el proceso deje de existir. Si sobrevive, se acaba con él al terminar la
// prueba para no dejarlo en la máquina.
func yaNoVive(t *testing.T, pid int, mensaje string) {
	t.Helper()
	eliminarAlTerminar(t, pid)
	require.Eventually(t, func() bool { return !sigueVivo(pid) }, 2*time.Second, 20*time.Millisecond, mensaje)
}

func TestComandos_UnComandoExitosoNoDejaProcesosEnSegundoPlano(t *testing.T) {
	pidFichero := filepath.Join(t.TempDir(), "pid")

	resultado := ejecutarSinCancelar(t, "sleep 60 & echo $! > "+pidFichero+"; echo listo")

	require.True(t, resultado.Exitoso)
	yaNoVive(t, leerPid(t, pidFichero), "el proceso en segundo plano sobrevivió a un comando que terminó bien")
}

func TestComandos_UnComandoQueFallaNoDejaProcesosEnSegundoPlano(t *testing.T) {
	pidFichero := filepath.Join(t.TempDir(), "pid")

	resultado := ejecutarSinCancelar(t, "sleep 60 & echo $! > "+pidFichero+"; exit 3")

	require.False(t, resultado.Exitoso)
	yaNoVive(t, leerPid(t, pidFichero), "el proceso en segundo plano sobrevivió a un comando que falló")
}

func TestComandos_TampocoSobreviveLoQueNoSeEnteraDeQueElComandoTermino(t *testing.T) {
	casos := map[string]string{
		// Sin la tubería de salida, Run vuelve en cuanto sale el shell: nada delata que quedó algo vivo.
		"un proceso que soltó la salida del comando": "sleep 60 >/dev/null 2>&1 </dev/null & echo $! > %s; echo listo",
		// Un nieto: lo lanzó un shell que el comando dejó a su vez en segundo plano, y que ya no es su padre.
		"un nieto": "(sh -c 'sleep 60 >/dev/null 2>&1 & echo $! > %s; wait' >/dev/null 2>&1 &); while [ ! -s %[1]s ]; do sleep 0.02; done; echo listo",
		// nohup y un trap protegen de SIGHUP y de SIGTERM, no de SIGKILL.
		"un proceso que ignora SIGTERM y SIGHUP": "(trap '' TERM HUP; exec sleep 60) >/dev/null 2>&1 & echo $! > %s; echo listo",
		"un proceso lanzado con nohup":           "nohup sleep 60 >/dev/null 2>&1 & echo $! > %s; echo listo",
	}
	for nombre, plantilla := range casos {
		t.Run(nombre, func(t *testing.T) {
			pidFichero := filepath.Join(t.TempDir(), "pid")

			resultado := ejecutarSinCancelar(t, fmt.Sprintf(plantilla, pidFichero))

			require.True(t, resultado.Exitoso)
			yaNoVive(t, leerPid(t, pidFichero), "sobrevivió al comando")
		})
	}
}

func TestComandos_UnComandoNoAcabaConLosProcesosDeOtro(t *testing.T) {
	// El grupo es el del comando, no el del motor: terminar uno no toca a quien no lanzó.
	ajeno := exec.Command("sleep", "60")
	require.NoError(t, ajeno.Start())
	t.Cleanup(func() { _ = ajeno.Process.Kill(); _ = ajeno.Wait() })

	resultado := ejecutarSinCancelar(t, "echo listo")

	require.True(t, resultado.Exitoso)
	require.True(t, sigueVivo(ajeno.Process.Pid), "un proceso del propio motor no es del grupo del comando")
}

func TestComandos_UnProcesoConSetsidSobrevive(t *testing.T) {
	// Documenta el límite (riesgo R2): un proceso que crea su propia sesión sale del grupo del comando, y el motor
	// no lo alcanza. La prueba existe para que el límite no se olvide, no porque esté bien.
	enOtraSesion := "setsid sleep 60"
	if _, err := exec.LookPath("setsid"); err != nil {
		// macOS no trae setsid: la misma llamada al sistema, desde perl.
		if _, err := exec.LookPath("perl"); err != nil {
			t.Skip("ni setsid ni perl están disponibles en esta máquina")
		}
		enOtraSesion = `perl -MPOSIX -e 'setsid(); exec "sleep", "60"'`
	}
	pidFichero := filepath.Join(t.TempDir(), "pid")

	// El comando espera a que el proceso haya salido de su grupo: si terminara antes, el proceso moriría con el
	// grupo sin haber llegado a escapar, y la prueba no diría nada del límite.
	grupoDe := func(pid string) string { return `"$(ps -o pgid= -p ` + pid + ` | tr -d ' ')"` }
	linea := enOtraSesion + " >/dev/null 2>&1 & p=$!; echo $p > " + pidFichero +
		"; while [ " + grupoDe("$p") + " = " + grupoDe("$$") + " ]; do sleep 0.02; done; echo listo"

	resultado := ejecutarSinCancelar(t, linea)

	require.True(t, resultado.Exitoso)
	pid := leerPid(t, pidFichero)
	eliminarAlTerminar(t, pid)
	require.Never(t, func() bool { return !sigueVivo(pid) }, 500*time.Millisecond, 50*time.Millisecond,
		"con setsid el proceso escapa del grupo: si esto cambia, hay que actualizar RD-13 y el riesgo R2")
}

// Un comando que escribe más de lo que se conserva no se queda bloqueado en su tubería: termina, y lo que se
// guarda dice que se truncó.
func TestComandos_UnComandoQueEscribeDeMasTerminaYSuSalidaQuedaTruncada(t *testing.T) {
	salida := dominio.NuevaSalidaDeComandoConLimite(1024)
	// 4 MiB: mucho más que el búfer de una tubería (64 KiB), que es donde se quedaría parado si no se vaciara.
	linea := "echo principio; head -c 4194304 /dev/zero | tr '\\0' x; echo; echo fin"
	terminado := make(chan struct{})
	var resultado dominio.ResultadoDeUnComando
	var err error
	go func() {
		defer close(terminado)
		resultado, err = infraestructura.NuevosComandos().Ejecutar(
			context.Background(), t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), dominio.Entorno{}, salida)
	}()

	select {
	case <-terminado:
	case <-time.After(30 * time.Second):
		t.Fatal("el comando se quedó bloqueado escribiendo: la tubería no se vació")
	}
	require.NoError(t, err)
	require.True(t, resultado.Exitoso, "escribir de más no es un fallo del comando")
	require.True(t, salida.Truncada())
	texto := salida.Texto()
	require.True(t, strings.HasPrefix(texto, "principio\nxxx"))
	require.Contains(t, texto, "[SALIDA TRUNCADA")
	require.NotContains(t, texto, "fin\n", "lo que pasa del límite no se conserva")
	require.Less(t, len(texto), 1024+200)
}

// Cada comando sabe dónde está el directorio efímero de su paso por su entorno, y puede usarlo.
func TestComandos_ElDirectorioEfimeroLlegaPorElEntornoYSePuedeUsar(t *testing.T) {
	efimero := infraestructura.NuevoDirectorioEfimero(filepath.Join(t.TempDir(), "vex-step-tmp"))
	ruta, err := efimero.Crear(context.Background())
	require.NoError(t, err)
	entorno := dominio.Entorno{}.Con(dominio.VariableDelDirectorioEfimero, ruta)
	salida := dominio.NuevaSalidaDeComando()
	linea := `mkdir "$VEX_STEP_TMP/azure" && echo sesion > "$VEX_STEP_TMP/azure/token" && echo "en=$VEX_STEP_TMP"`

	resultado, err := infraestructura.NuevosComandos().Ejecutar(
		context.Background(), t.TempDir(), linea, comandoDeclarado(t, linea, nil, nil), entorno, salida)

	require.NoError(t, err)
	require.True(t, resultado.Exitoso, salida.Texto())
	require.Contains(t, salida.Texto(), "en="+ruta)
	require.FileExists(t, filepath.Join(ruta, "azure", "token"))

	require.NoError(t, efimero.Borrar(context.Background()))
	require.NoDirExists(t, ruta, "al terminar el paso no queda nada de la sesión")
}
