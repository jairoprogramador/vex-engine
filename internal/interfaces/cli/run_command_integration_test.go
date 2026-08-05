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
	"encoding/base64"
	"strings"
	"testing"

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

func TestRunCommand_PasoInexistente(t *testing.T) {
	h := newHarness(t)

	result := h.run(withStep("promote"))

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "no está definido en el pipeline")
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
