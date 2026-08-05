package cli_test

// Harness de integración sobre el cableado (spec 01).
//
// Cada caso ejecuta `RunCommand.Execute` con el MISMO cableado que arma el
// binario —tres cadenas anidadas, policy, almacén y todos los repositorios de
// archivo— contra un $HOME temporal. Lo que se prueba no son los handlers uno a
// uno (para eso está la spec 00) sino que estén conectados en el orden correcto
// y con el repositorio correcto en cada rama.
//
// El andamiaje está en harness_test.go.

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// correHastaEstable ejecuta hasta que la policy deja de mandar ejecutar. Hoy
// hacen falta DOS ejecuciones: ver TestRunCommand_ReejecucionSinCambios.
func correHastaEstable(t *testing.T, h *harness) {
	t.Helper()
	for i := 0; i < 5; i++ {
		h.resetLog()
		result := h.run()
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		if len(h.ranSteps()) == 0 {
			return
		}
	}
	t.Fatal("la pipeline nunca llegó a un estado estable")
}

// ── Ejecución completa ──────────────────────────────────────────────────────

func TestRunCommand_EjecucionLimpia(t *testing.T) {
	h := newHarness(t)

	result := h.run(withStep("supply"), withEnvironment("sand"))

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Empty(t, result.stderr)

	// `supply` es el segundo step: la cadena de pipeline ejecuta todos los
	// anteriores, en orden.
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

	// El valor demuestra dos cosas que ningún test unitario ve:
	//   - `artifact_name` lo extrajo 01-test y sobrevivió hasta 02-supply: el
	//     mapa acumulado cruza steps.
	//   - `registry_prefix` salió de variables/sand/supply.yaml: la rama de
	//     ambiente del repositorio de variables está bien inyectada.
	assert.Equal(t, `02-supply acr_name = "vexsand-demo-app"`, h.logLines()[1])

	// Y que el almacén de variables persistió lo extraído.
	stored := h.storedVars("sand", "supply")
	assert.Equal(t, "vexsand-demo-app", stored["acr_name"])
	assert.Equal(t, "demo-app", stored["artifact_name"])

	// Las variables volátiles NO entran al almacén (step_executable.go).
	assert.NotContains(t, stored, "project_version")
	assert.NotContains(t, stored, "project_revision")
	assert.NotContains(t, stored, "step_workdir")
}

// ── Persistencia de la policy y del almacén ─────────────────────────────────

func TestRunCommand_ReejecucionSinCambios(t *testing.T) {
	h := newHarness(t)

	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

	// DIVERGENCIA VIVA respecto de lo que la spec 01 §5.2 daba por hecho: la
	// segunda ejecución idéntica NO se salta.
	//
	// Motivo: cada step guarda en el almacén las variables que él mismo
	// produjo (`artifact_name`, `acr_name`). La ejecución siguiente las carga
	// ANTES de evaluar la policy, así que la huella de variables que ve la
	// regla ya no es la que se guardó, y el step se re-ejecuta exactamente una
	// vez más. A partir de la tercera el punto es fijo.
	//
	// Este test afirma el comportamiento ACTUAL. Las specs 12/14/20 —el literal
	// como default y el consumidor declarando el origen— deberían volverlo
	// verde en la segunda ejecución; cuando pase, este test se actualiza y el
	// diff hace visible la corrección.
	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps(),
		"la segunda ejecución todavía re-ejecuta: las variables de salida del propio step cambian su huella")

	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.Empty(t, h.ranSteps(), "la tercera ejecución sí se salta: policy y almacén persisten y se leen")

	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.Empty(t, h.ranSteps())
}

func TestRunCommand_CambioEnElCodigoDelProyecto(t *testing.T) {
	h := newHarness(t)
	correHastaEstable(t, h)

	// Un byte del proyecto. En modo local el motor enlaza el directorio en vez
	// de clonarlo, así que la huella de código ve el árbol de trabajo.
	h.writeProjectFile("src/app.txt", "v2\n")

	h.resetLog()
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	// `test` tiene la regla de código; `supply` no (policy_builder.go). El
	// cambio re-ejecuta uno y deja el otro saltado.
	assert.Equal(t, []string{"01-test"}, h.ranSteps())

	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.Empty(t, h.ranSteps())
}

// ── Huella de contenido (spec 08) ───────────────────────────────────────────

// Un `chmod +x` cambia lo que pasa al desplegar. Antes de la spec 08 no cambiaba
// la huella: el motor concluía «el código no cambió» y saltaba el step. Era un
// falso negativo del caché con efecto en producción.
func TestRunCommand_UnChmodEnElProyectoReejecutaElStep(t *testing.T) {
	h := newHarness(t)
	h.writeProjectFile("scripts/deploy.sh", "#!/bin/sh\necho desplegando\n")
	correHastaEstable(t, h)

	h.chmodProjectFile("scripts/deploy.sh", 0o755)

	h.resetLog()
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	// `test` es el step con la regla de código; `supply` no la tiene.
	assert.Equal(t, []string{"01-test"}, h.ranSteps())

	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.Empty(t, h.ranSteps(), "el permiso ya está en la huella persistida")
}

// Un enlace dentro del proyecto era invisible a la identidad. Ahora su destino
// es material de identidad, sin que el enlace se siga.
func TestRunCommand_CambiarElDestinoDeUnEnlaceReejecutaElStep(t *testing.T) {
	h := newHarness(t)
	h.writeProjectFile("src/a.txt", "a\n")
	h.writeProjectFile("src/b.txt", "b\n")
	h.symlinkProjectFile("src/actual.txt", "a.txt")
	correHastaEstable(t, h)

	h.symlinkProjectFile("src/actual.txt", "b.txt")

	h.resetLog()
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	assert.Equal(t, []string{"01-test"}, h.ranSteps())
}

// La huella que se persiste y se compara lleva el prefijo de la regla. Sin él,
// corregir una divergencia congelada en una v2 sería indistinguible de un bug.
func TestRunCommand_LaHuellaPersistidaLlevaLaVersionDeLaRegla(t *testing.T) {
	h := newHarness(t)
	correHastaEstable(t, h)

	huella := h.storedCodeFingerprint("test")

	assert.Regexp(t, `^v1:[0-9a-f]{64}$`, huella)
}

// ── Aislamiento de ambiente (regresión de R-22) ─────────────────────────────

func TestRunCommand_AmbientesAislados(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run(withEnvironment("sand")).exitCode)
	assert.Equal(t, "vexsand-demo-app", h.storedVars("sand", "supply")["acr_name"])

	h.resetLog()
	result := h.run(withEnvironment("prod"))
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	// R-22: `supply@prod` no se salta pese a que `supply@sand` acaba de correr.
	// Hoy pasa por accidente —la regla de instrucciones NO lleva el ambiente en
	// su clave, la de variables sí—, y este test lo convierte en contrato.
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	assert.Equal(t, "vexprod-demo-app", h.storedVars("prod", "supply")["acr_name"])

	// Los dos almacenes conviven: prod no pisó a sand.
	assert.Equal(t, "vexsand-demo-app", h.storedVars("sand", "supply")["acr_name"])
}

// ── Precedencia de variables (canario del orden de chainStepHandlers) ───────

func TestRunCommand_PrecedenciaDeclaradoSobreAlmacenado(t *testing.T) {
	h := newHarness(t)
	correHastaEstable(t, h)
	require.Equal(t, "vexsand-demo-app", h.storedVars("sand", "supply")["acr_name"])

	// El almacén tiene registry_prefix=vexsand. El pipelinecode ahora declara
	// otro valor: gana lo declarado porque los handlers 01/02 (almacén) corren
	// ANTES que el 03 (variables declaradas) y el mapa es "el último gana".
	h.commitPipelineFile("variables/sand/supply.yaml", "- name: registry_prefix\n  value: vexsand2\n")

	h.resetLog()
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	// CANARIO: intercambiar dos handlers en chainStepHandlers pone esto en rojo.
	assert.Equal(t, []string{"02-supply"}, h.ranSteps())
	assert.Equal(t, "vexsand2-demo-app", h.storedVars("sand", "supply")["acr_name"])
}

// ── Invariante de variable (spec 03) ────────────────────────────────────────

func TestRunCommand_VariableDeclaradaSinValor(t *testing.T) {
	// El pipelinecode declara un parámetro sin valor. Hasta la spec 03 la
	// ejecución entera fallaba: `NewVariable` rechazaba el valor vacío y el
	// repositorio de variables propagaba el error. Con el invariante partido
	// (§5.1) «declarada y vacía» es un dato legítimo, y `${var.instance_count}`
	// interpola a cadena vacía en vez de ser inexpresable (§5.4).
	h := newHarness(t,
		withPipelineFile("variables/sand/supply.yaml",
			"- name: registry_prefix\n  value: vexsand\n- name: instance_count\n  value: \"\"\n"),
		withPipelineFile("steps/02-supply/commands.yaml", `
- name: provision
  cmd: echo '02-supply acr_name = "${var.registry_prefix}-${var.artifact_name}"' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: acr_name
      probe: acr_name = "([^"]+)"
- name: escalar
  cmd: echo '02-supply instancias=[${var.instance_count}]' | tee -a "$VEX_TEST_LOG"
`))

	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, "02-supply instancias=[]", h.logLines()[2],
		"la variable declarada y vacía interpola a cadena vacía")

	// El almacén distingue «declarada y vacía» de «no declarada»: la clave
	// existe con valor vacío. Y no aparece la entrada anónima —eso lo comprueba
	// además `assertNingunaVariableAnonima` tras cada ejecución del harness.
	stored := h.storedVars("sand", "supply")
	assert.NotContains(t, stored, "")
	require.Contains(t, stored, "instance_count")
	assert.Equal(t, "", stored["instance_count"])
}

func TestRunCommand_CampoObligatorioDelProyectoSigueSiendoObligatorio(t *testing.T) {
	// DIVERGENCIA con la spec 03 §1 y §7: el disparador que describe —«un equipo
	// sin asignar» llegando a `07_init_vars` y produciendo la entrada anónima—
	// no es alcanzable desde el input, porque `create_execution.go:74-100`
	// rechaza antes los nueve campos del RequestInput. La entrada anónima que la
	// spec corrige solo era alcanzable por un defecto del propio motor.
	//
	// Aflojar esa validación es lo que habilitará el parámetro opcional de D-A8,
	// y no está en el alcance de esta spec (§6). Este test fija el borde actual:
	// cuando se afloje, se pone en rojo y la decisión se hace visible.
	h := newHarness(t)

	result := h.run(withProjectTeam(""))

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "project team is required")
	assert.Empty(t, h.ranSteps())
}

// ── Fallos dentro de la cadena de comando ───────────────────────────────────

func TestRunCommand_ComandoConExitCodeDistintoDeCero(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/01-test/commands.yaml", `
- name: falla
  cmd: echo "01-test antes-del-fallo" | tee -a "$VEX_TEST_LOG"; exit 3
- name: siguiente
  cmd: echo "01-test NO-DEBE-CORRER" | tee -a "$VEX_TEST_LOG"
`))

	result := h.run()

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "exit code 3")

	// Fail-fast: el comando siguiente del mismo step no se ejecuta, y el step
	// posterior (02-supply) tampoco.
	assert.Equal(t, []string{"01-test"}, h.ranSteps())
	assert.Equal(t, "01-test antes-del-fallo", h.logLines()[0])
}

// ── Limpieza tras un fallo (spec 06) ────────────────────────────────────────

// La plantilla se interpola EN EL SITIO dentro de la copia de trabajo, y el
// original se restaura al terminar el step. Hasta la spec 06 esa restauración
// vivía detrás del camino feliz: cuando el comando fallaba —el caso normal— la
// copia quedaba con los valores sustituidos.
func TestRunCommand_UnComandoFallidoDejaElWorkdirLimpio(t *testing.T) {
	const plantilla = "steps/01-test/k8s/deployment.yaml"
	const original = "image: ${var.environment}-${var.project_name}\n"

	h := newHarness(t,
		withPipelineFile(plantilla, original),
		withPipelineFile("steps/01-test/commands.yaml", `
- name: falla
  cmd: cat "${var.step_workdir}/k8s/deployment.yaml" | tee -a "$VEX_TEST_LOG"; exit 3
  templates:
    - k8s/deployment.yaml
`))

	primera := h.run()
	require.Equal(t, cli.ExitFailed, primera.exitCode)

	// El comando SÍ vio la plantilla interpolada: es para eso que se interpola.
	assert.Equal(t, "image: sand-demo-app", h.logLines()[0])

	// Y al salir, la copia de trabajo volvió a su contenido original.
	assert.Equal(t, original, h.workdirFile(plantilla),
		"una plantilla que se queda interpolada ya no tiene ${var.…} que interpolar en la corrida siguiente")

	// Corolario: el segundo intento falla igual que el primero. Hoy la copia del
	// workdir se rehace desde el clon en cada ejecución, así que esta parte
	// pasaba también antes de la spec 06; queda como guardia de que la
	// restauración no introduce una diferencia entre corridas.
	h.resetLog()
	segunda := h.run()
	assert.Equal(t, cli.ExitFailed, segunda.exitCode)
	assert.Equal(t, "image: sand-demo-app", h.logLines()[0])
	assert.Equal(t, original, h.workdirFile(plantilla))
}

func TestRunCommand_ProbeSinCoincidencia(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/01-test/commands.yaml", `
- name: build
  cmd: echo "01-test BUILD SUCCESS" | tee -a "$VEX_TEST_LOG"
  outputs:
    - probe: NUNCA-APARECE-EN-STDOUT
`))

	result := h.run()

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "no encontró coincidencia")
	assert.Equal(t, []string{"01-test"}, h.ranSteps())
}

func TestRunCommand_OutputConGrupoVacio(t *testing.T) {
	// El probe SÍ casa —así que el handler 04 lo deja pasar— pero el grupo 1
	// sale vacío y el 05 no puede construir la variable.
	h := newHarness(t, withPipelineFile("steps/02-supply/commands.yaml", `
- name: provision
  cmd: echo '02-supply acr_name = ""' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: acr_name
      probe: acr_name = "([^"]*)"
`))

	result := h.run()

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "valor vacío")
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

	// El step falló: su estado se borra para que la próxima ejecución lo
	// reintente en vez de saltarlo.
	assert.Empty(t, h.storedVars("sand", "supply"))
}

// ── Validación del pipelinecode (spec 04) ───────────────────────────────────
//
// Los cinco casos de esta sección producían HOY —antes de la spec 04— una
// ejecución con exit code 0 y un step de menos, o los steps reordenados. Lo que
// se observa no es solo que fallen: es que fallan ANTES del primer step, así que
// ningún despliegue queda a medias, y que el mensaje nombra al culpable.

func TestRunCommand_EstructuraDeStepsInvalida(t *testing.T) {
	casos := []struct {
		nombre    string
		archivo   string
		enElError string
		nota      string
	}{
		{
			nombre:    "prefijo de un dígito",
			archivo:   "steps/2-promote/commands.yaml",
			enElError: "steps/2-promote",
			nota:      "HOY el step pasa sin ejecutar nada Y reordena los demás",
		},
		{
			nombre:    "separador guion bajo",
			archivo:   "steps/4_test/commands.yaml",
			enElError: "steps/4_test",
			nota:      "HOY desaparece del pipeline en silencio",
		},
		{
			nombre:    "dos steps con el mismo orden",
			archivo:   "steps/02-otro/commands.yaml",
			enElError: "orden 02",
			nota:      "HOY el orden entre los dos es arbitrario y comparten clave de estado",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			h := newHarness(t, withPipelineFile(caso.archivo,
				"- name: ruido\n  cmd: echo 'NO-DEBE-CORRER' | tee -a \"$VEX_TEST_LOG\"\n"))

			result := h.run()

			assert.Equal(t, cli.ExitFailed, result.exitCode, caso.nota)
			assert.Contains(t, result.stderr, "estructura del pipelinecode inválida")
			assert.Contains(t, result.stderr, caso.enElError, "el error nombra al culpable")
			assert.Empty(t, h.ranSteps(),
				"la validación corre antes del primer step: ningún despliegue queda a medias")
		})
	}
}

func TestRunCommand_AmbienteLlamadoShared(t *testing.T) {
	// `shared` es el ámbito del almacén compartido y ocupa la misma posición que
	// el ambiente en la ruta del almacén: declararlo como ambiente lo pisaría
	// (spec 04 §5.4).
	h := newHarness(t, withPipelineFile("environments.yaml",
		"- name: Compartido\n  value: shared\n- name: Sandbox\n  value: sand\n"))

	result := h.run(withEnvironment("sand"))

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "'shared' está reservado")
	assert.Empty(t, h.ranSteps(),
		"se rechaza el environments.yaml entero, aunque el ambiente pedido sea otro")
}

func TestRunCommand_StepSinComandosNiSeEjecutaNiPersisteEstado(t *testing.T) {
	// D-A12: un `commands.yaml` vacío es `skipped{no_commands}`, no `success`.
	// El efecto dañino real no era el vocabulario sino que el step persistía
	// estado, dejando escrito «sin cambios» sobre cero comandos ejecutados.
	// Control de que la aserción de abajo observa algo: un step con comandos sí
	// deja estado, así que su ausencia en el caso vacío es la diferencia.
	control := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, control.run().exitCode)
	require.NotEmpty(t, control.persistedStepState("supply"))

	h := newHarness(t, withPipelineFile("steps/02-supply/commands.yaml", ""))

	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, []string{"01-test"}, h.ranSteps(),
		"02-supply no ejecutó ningún comando")
	assert.Empty(t, h.persistedStepState("supply"),
		"un step saltado por falta de comandos no deja estado de re-ejecución")
	assert.Empty(t, h.storedVars("sand", "supply"),
		"tampoco el almacén: registry_prefix estaba declarado, pero nada lo consumió")

	// Segunda corrida: vuelve a saltarse por la MISMA razón. Si hubiera
	// persistido estado, se saltaría por caché y las dos serían indistinguibles.
	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.NotContains(t, h.ranSteps(), "02-supply")
	assert.Empty(t, h.persistedStepState("supply"))
}

// ── Evaluar no escribe: el estado se persiste tras el éxito (spec 09) ───────

// Hasta la spec 09 las reglas escribían la huella nueva dentro de `Evaluate`,
// antes del primer comando del step, y el borrado compensatorio la quitaba si el
// step fallaba. Este caso —un fallo ordinario, que sí pasa por el camino de error
// de Go— ya estaba cubierto por aquel compensador y por tanto ya era verde.
//
// Lo que fija ahora es que sigue siéndolo SIN compensador: no hay estado que
// revertir porque no se escribió ninguno. Es la red que impide que la escritura
// anticipada vuelva por otra vía, y el único observable de disco que el harness
// puede dar sobre esto: la muerte dura —SIGKILL, OOM, un corte de luz— no se
// puede montar aquí, y es justo el caso que ningún compensador podía cubrir.
func TestRunCommand_UnStepQueNoTerminaNoDejaEstadoDeReejecucion(t *testing.T) {
	const supplyCmd = "steps/02-supply/commands.yaml"
	const supplyRoto = `
- name: provision
  cmd: echo "02-supply INTENTO" | tee -a "$VEX_TEST_LOG"
- name: reventar
  cmd: exit 1
`

	h := newHarness(t, withPipelineFile(supplyCmd, supplyRoto))

	result := h.run()

	require.Equal(t, cli.ExitFailed, result.exitCode, result.stderr)
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

	assert.NotEmpty(t, h.persistedStepState("test"),
		"control: el step que SÍ terminó deja su estado, así que la ausencia de abajo se ve")
	assert.Empty(t, h.persistedStepState("supply"),
		"el step que empezó y no terminó no deja nada escrito")

	// Y no es cosa de una corrida: la siguiente vuelve a intentarlo en vez de
	// encontrar una huella escrita y saltárselo.
	h.resetLog()
	segunda := h.run()
	assert.Equal(t, cli.ExitFailed, segunda.exitCode)
	assert.Contains(t, h.ranSteps(), "02-supply",
		"sin estado persistido, el step roto se reintenta")
}

// El otro lado del mismo hecho: el estado aparece DESPUÉS del éxito, no antes.
func TestRunCommand_ElEstadoAparecTrasElExitoDelStep(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	assert.NotEmpty(t, h.persistedStepState("test"))
	assert.NotEmpty(t, h.persistedStepState("supply"))
}

// ── Step con nombre desconocido (spec 05) ───────────────────────────────────

func TestRunCommand_StepDesconocidoFallaEnVezDeSaltarse(t *testing.T) {
	// `05-notify` es estructuralmente impecable —prefijo de dos dígitos, orden
	// único, comandos declarados— y hasta la spec 05 la ejecución terminaba con
	// exit code 0 sin haber corrido un solo comando suyo: `PolicyBuilder` le daba
	// cero reglas, y una policy sin reglas concluía «all rules passed».
	//
	// Ahora el motor dice que no sabe evaluarlo, y lo dice fallando.
	const notifyCmd = "steps/05-notify/commands.yaml"
	const notifyBody = `
- name: avisar
  cmd: echo "05-notify NO-DEBE-CORRER" | tee -a "$VEX_TEST_LOG"
`

	h := newHarness(t, withPipelineFile(notifyCmd, notifyBody))

	result := h.run(withStep("notify"))

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "no tiene comprobaciones definidas")
	assert.Contains(t, result.stderr, "notify", "el error nombra al step")
	assert.Contains(t, result.stderr, "test, supply, package, deploy",
		"y enumera los conocidos, que es lo accionable")

	// El fallo es del step desconocido, no de los anteriores: los dos que el
	// motor sí sabe evaluar corrieron, y el suyo no ejecutó nada.
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	assert.Empty(t, h.persistedStepState("notify"),
		"un step que el motor no sabe evaluar no deja estado de re-ejecución")

	// Y no es un fallo de una sola corrida: la siguiente vuelve a fallar igual,
	// en vez de encontrar una huella escrita y saltarse el problema.
	h.resetLog()
	segunda := h.run(withStep("notify"))
	assert.Equal(t, cli.ExitFailed, segunda.exitCode)
	assert.Contains(t, segunda.stderr, "no tiene comprobaciones definidas")
}

func TestRunCommand_StepDesconocidoPosteriorAlPedidoNoSeDiagnostica(t *testing.T) {
	// LÍMITE del arreglo, medido y no supuesto. El diagnóstico es PEREZOSO: vive
	// en la cadena 2, así que solo alcanza a los steps que la corrida toca. La
	// cadena de pipeline ejecuta `steps[:pedido+1]`, de modo que un `05-notify`
	// detrás de `02-supply` no se construye, no se evalúa y no se diagnostica.
	//
	// El pipelinecode roto sigue existiendo y `vex supply` sigue devolviendo 0. Lo
	// que cambia es que deja de haber una corrida que *parezca* haber ejecutado el
	// step: para verlo hay que pedirlo. Adelantar la comprobación al validador de
	// estructura (spec 04) queda EXPLÍCITAMENTE fuera de la spec 05 §6, y la 10 lo
	// vuelve innecesario: al borrar el switch, un step desconocido se ejecuta.
	h := newHarness(t, withPipelineFile("steps/05-notify/commands.yaml",
		"- name: avisar\n  cmd: echo \"05-notify NO-DEBE-CORRER\" | tee -a \"$VEX_TEST_LOG\"\n"))

	result := h.run(withStep("supply"))

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
}

func TestRunCommand_StepDesconocidoSinComandosSigueSiendoNoCommands(t *testing.T) {
	// Frontera entre la spec 04 y la 05, y el orden importa: el handler comprueba
	// PRIMERO que haya comandos y solo entonces construye la policy. Un
	// `05-notify` con el archivo vacío es `skipped{no_commands}` —un resultado con
	// razón, emitido— y no llega a preguntar por sus reglas.
	//
	// No es el silencio que corrige la 05: ahí no hay nada que ejecutar ni, por
	// tanto, nada que decidir.
	h := newHarness(t, withPipelineFile("steps/05-notify/commands.yaml", ""))

	result := h.run(withStep("notify"))

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	assert.Empty(t, h.persistedStepState("notify"))
}

func TestRunCommand_PasoInexistente(t *testing.T) {
	h := newHarness(t)

	result := h.run(withStep("promote"))

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "no está definido en el pipeline")
	assert.Empty(t, h.ranSteps())
}

// ── Ciclo de vida de la ejecución (spec 07) ─────────────────────────────────

// Un Ctrl-C es la forma normal de abortar en local, y hasta la spec 07 mataba
// el proceso sin dejar rastro: no había manejador de señales y el estado
// terminal lo deducía esta capa a partir del error. Ahora la interrupción
// cancela el contexto, el agregado la registra como `canceled` y el proceso
// sale con un código que NO se confunde con el de una pipeline fallida.
//
// Lo que el test manda no es la señal sino lo que la señal provoca —cancelar
// el contexto—; mandarle un SIGINT de verdad mataría el proceso de test.
//
// La salida no es instantánea: cancelar mata el `sh` del comando, pero sus
// hijos heredan el pipe de stdout y la espera no termina hasta que ellos
// también salen. Es exactamente la razón por la que la cancelación es
// best-effort y por la que el manejador de señales lleva un plazo (§5.4).
func TestRunCommand_UnaCancelacionDuranteUnComandoNoEsUnFallo(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/01-test/commands.yaml", `
- name: comando largo
  cmd: echo "01-test arrancado" | tee -a "$VEX_TEST_LOG"; sleep 5
- name: siguiente
  cmd: echo "01-test NO-DEBE-CORRER" | tee -a "$VEX_TEST_LOG"
`))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancelar en cuanto el comando dio señales de vida: antes sería cancelar
	// la clonación, que es otro caso (el de abajo).
	go func() {
		for i := 0; i < 300; i++ {
			if len(h.logLines()) > 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		cancel()
	}()

	args := h.args()
	args.InputFile = h.writeRequest(h.request())
	result := h.executeCtx(ctx, args, nil)

	assert.Equal(t, cli.ExitCancelled, result.exitCode,
		"cancelar es una decisión; fallar es otra cosa, y el exit code las distingue")
	assert.Contains(t, result.stderr, "cancelled")
	assert.Equal(t, []string{"01-test"}, h.ranSteps())

	// Y la cancelación pasa por el camino de error, no por encima de él: la
	// limpieza de la spec 06 corre y el borrado compensatorio de las huellas
	// también. Un `Ctrl-C` deja de ser muerte dura, que es lo que estrecha la
	// ventana descrita en la spec 09 §1 (a).
	assert.Empty(t, h.persistedStepState("test"),
		"un step cancelado no puede dejar escrito «sin cambios»")
}

// La señal puede llegar antes de que la cadena alcance el primer comando. El
// resultado es el mismo: cancelada, no fallida.
func TestRunCommand_UnaCancelacionAntesDeEmpezarTampocoEsUnFallo(t *testing.T) {
	h := newHarness(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	args := h.args()
	args.InputFile = h.writeRequest(h.request())
	result := h.executeCtx(ctx, args, nil)

	assert.Equal(t, cli.ExitCancelled, result.exitCode)
	assert.Empty(t, h.ranSteps())
}

// ── Validación del input (exit code 2) ──────────────────────────────────────

func TestRunCommand_SchemaVersionNoSoportada(t *testing.T) {
	h := newHarness(t)

	result := h.run(withSchemaVersion(99))

	assert.Equal(t, cli.ExitInputError, result.exitCode)
	assert.Contains(t, result.stderr, "unsupported schema_version: 99")
	assert.Empty(t, h.ranSteps())
}

func TestRunCommand_InputVacio(t *testing.T) {
	h := newHarness(t)

	t.Run("sin ninguna fuente", func(t *testing.T) {
		result := h.execute(h.args(), nil)
		assert.Equal(t, cli.ExitInputError, result.exitCode)
		assert.Contains(t, result.stderr, "no input source")
	})

	t.Run("stdin vacío", func(t *testing.T) {
		result := h.execute(h.args(), strings.NewReader(""))
		assert.Equal(t, cli.ExitInputError, result.exitCode)
		assert.Contains(t, result.stderr, "no input source")
	})

	t.Run("JSON malformado", func(t *testing.T) {
		result := h.execute(h.args(), strings.NewReader("{no soy json"))
		assert.Equal(t, cli.ExitInputError, result.exitCode)
		assert.Contains(t, result.stderr, "parse input")
	})

	assert.Empty(t, h.ranSteps())
}

// ── Las tres vías de input (§5.3) ───────────────────────────────────────────
//
// Un caso por vía. La spec 16 añade otra flag a este mismo camino, así que
// conviene que el existente esté cubierto antes de tocarlo.

func TestRunCommand_ViasDeInput(t *testing.T) {
	t.Run("--input", func(t *testing.T) {
		h := newHarness(t)
		result := h.run()
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	})

	t.Run("env var con JSON crudo", func(t *testing.T) {
		h := newHarness(t)
		t.Setenv("VEX_REQUEST_INPUT", string(h.marshal(h.request())))

		result := h.execute(h.args(), nil)
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	})

	t.Run("env var con base64", func(t *testing.T) {
		h := newHarness(t)
		t.Setenv("VEX_REQUEST_INPUT", base64.StdEncoding.EncodeToString(h.marshal(h.request())))

		result := h.execute(h.args(), nil)
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	})

	t.Run("env var alternativa vía --input-env", func(t *testing.T) {
		h := newHarness(t)
		t.Setenv("OTRA_VAR", string(h.marshal(h.request())))

		args := h.args()
		args.InputEnv = "OTRA_VAR"
		result := h.execute(args, nil)
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	})

	t.Run("stdin", func(t *testing.T) {
		h := newHarness(t)

		result := h.execute(h.args(), strings.NewReader(string(h.marshal(h.request()))))
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	})

	t.Run("--input gana sobre la env var", func(t *testing.T) {
		h := newHarness(t)
		t.Setenv("VEX_REQUEST_INPUT", string(h.marshal(h.request(withSchemaVersion(99)))))

		// El archivo trae un input válido; si la env var ganara, el exit code
		// sería 2.
		result := h.run()
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	})
}

// ── Composición de observers ────────────────────────────────────────────────

func TestRunCommand_ObserverDeStatusNuncaRecibeStages(t *testing.T) {
	// CARACTERIZACIÓN: con --quiet=false, RunCommand compone un
	// StdoutStatusObserver sobre el writer que recibe... y nada lo notifica
	// nunca. `ExecutionContext.NotifyStage` existe y `PipelineRequestHandler`
	// lo expone, pero ningún handler lo llama.
	//
	// El writer se queda vacío. Cuando el registro de despliegue (specs 17-19)
	// empiece a emitir hechos, este test se pone en rojo, que es justo lo que
	// se quiere: que la aparición de stages sea una decisión visible.
	h := newHarness(t)

	args := h.args()
	args.Quiet = false
	result := h.execute(args, strings.NewReader(string(h.marshal(h.request()))))

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Empty(t, result.stdout, "ningún handler llama a NotifyStage")
}
