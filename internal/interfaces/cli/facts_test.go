package cli_test

// La emisión de hechos (spec 19), verificada contra el cableado REAL.
//
// Estos casos son la §7 de esa spec y viven aparte de
// `run_command_integration_test.go` por lo mismo que los de la 18: hablan de
// quién emite qué, no de lo que las specs anteriores fijaron. El montaje es el
// mismo harness, y los hechos se leen del JSONL con el DTO real — lo que el
// motor escribe es lo que un ingestor de otra plataforma va a leer, y una prueba
// que reconstruyera el evento por otra vía no estaría comprobando eso.
//
// Lo que estos casos NO pueden probar en memoria —y por eso están aquí y no en
// `internal/domain/record`— es lo único que sólo se ve con el motor entero: que
// haya UN dueño por hecho, que el par se cierre siempre, y que el pliegue
// coincida con lo que de verdad pasó.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// ── Un solo dueño ───────────────────────────────────────────────────────────

// UN `step_started` Y UN `step_finished` POR STEP. Ni duplicados ni huecos.
//
// Es la razón de que la alternativa A —«emitir desde donde haga falta»— se
// descartara: dos capas ven terminar un step, `StepExecutable` y
// `StepRunnerHandler`, y ninguna sabe si la otra emitió. El dueño se declara, y
// esto es esa declaración hecha ejecutable.
func TestHechos_UnSoloDuenoPorHecho(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	abiertos := idsDeStep(h.hechosDeTipo(record.TypeStepStarted.String()))
	cerrados := idsDeStep(h.hechosDeTipo(record.TypeStepFinished.String()))

	assert.Equal(t, []string{"01-test", "02-supply"}, abiertos)
	assert.Equal(t, abiertos, cerrados, "cada apertura tiene su cierre, y sólo uno")

	// Y el intento se abre y se cierra una vez.
	assert.Len(t, h.hechosDeTipo(record.TypeAttemptStarted.String()), 1)
	assert.Len(t, h.hechosDeTipo(record.TypeAttemptFinished.String()), 1)
}

// LOS HECHOS A NIVEL COMANDO (A2 resuelta que sí).
//
// El coste es volumen y está acotado: el fixture tiene un comando por step, los
// tres templates tienen del orden de seis, así que un `deploy` produce decenas
// de hechos y no miles. A cambio se responde «¿qué comando falló y cuánto
// tardó?», que es la primera pregunta de cualquier diagnóstico. Recortar a nivel
// step se puede hacer después; recuperar los meses anteriores, no.
func TestHechos_CadaComandoAbreYCierraElSuyo(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	abiertos := h.hechosDeTipo(record.TypeCommandStarted.String())
	cerrados := h.hechosDeTipo(record.TypeCommandFinished.String())

	require.Len(t, abiertos, 2)
	require.Len(t, cerrados, 2)

	assert.Equal(t, "build", abiertos[0].Payload["command"])
	assert.Equal(t, "01-test", abiertos[0].Payload["step_id"])
	assert.Equal(t, "provision", abiertos[1].Payload["command"])

	// El status deja de ser un campo que se asigna y nadie lee (BL-4), y el exit
	// code deja de morir dentro de su handler (BL-30).
	assert.Equal(t, command.CommandSuccess.String(), cerrados[0].Payload["status"])
	assert.Equal(t, float64(0), cerrados[0].Payload["exit_code"])
	assert.NotContains(t, cerrados[0].Payload, "error_class",
		"un comando que fue bien no reporta clase de error")
}

// ── El par siempre se cierra ────────────────────────────────────────────────

// UN COMANDO QUE FALLA PRODUCE SU CIERRE, con su exit code y su clase.
//
// **Es el test que depende de la spec 06**: sin el `defer` que aquélla puso, el
// ciclo del step fallido no llegaba a su cierre y el hecho que más importa —el
// del fallo— no se emitía.
func TestHechos_UnComandoQueFallaCierraSuParConSuExitCode(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/01-test/commands.yaml",
		"- name: build\n  cmd: exit 7\n"))

	require.Equal(t, cli.ExitFailed, h.run().exitCode)

	cerrados := h.hechosDeTipo(record.TypeCommandFinished.String())
	require.Len(t, cerrados, 1)
	assert.Equal(t, command.CommandFailure.String(), cerrados[0].Payload["status"])
	assert.Equal(t, float64(7), cerrados[0].Payload["exit_code"])
	assert.Equal(t, record.ErrorClassCommandFailed.String(), cerrados[0].Payload["error_class"])

	// Y el step que lo contenía también cierra, con el mismo código: el exit code
	// del step es el del comando que lo tumbó.
	steps := h.hechosDeTipo(record.TypeStepFinished.String())
	require.Len(t, steps, 1)
	assert.Equal(t, command.StepFailure.String(), steps[0].Payload["status"])
	assert.Equal(t, float64(7), steps[0].Payload["exit_code"])

	// El step siguiente no llega a abrirse: el fallo aborta la cadena.
	assert.Equal(t, []string{"01-test"}, idsDeStep(h.hechosDeTipo(record.TypeStepStarted.String())))
}

// ── Duraciones ──────────────────────────────────────────────────────────────

// TODOS LOS `*_finished` TRAEN `duration_ms`. Es el campo con más valor del
// vocabulario y hasta la spec 19 no existía a ningún nivel.
//
// Se afirma que el campo ESTÁ y que no es negativo, no que sea mayor que cero:
// el reloj es el del sistema y un `echo` puede caer dentro del mismo
// milisegundo. Exigir `> 0` haría el caso intermitente sin medir nada más.
func TestHechos_LosCierresTraenSuDuracion(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	for _, tipo := range []string{
		record.TypeStepFinished.String(),
		record.TypeCommandFinished.String(),
	} {
		hechos := h.hechosDeTipo(tipo)
		require.NotEmpty(t, hechos, tipo)
		for _, hecho := range hechos {
			duracion, ok := hecho.Payload["duration_ms"]
			require.True(t, ok, "%s sin duration_ms", tipo)
			assert.GreaterOrEqual(t, duracion, float64(0))
		}
	}
}

// ── El salto registrado, que es la pregunta sin respuesta ───────────────────

// UN STEP QUE REVIVE PRODUCE `step_finished{from_cache: true}` CON SU MOTIVO Y
// SU EVIDENCIA, y con eso «¿cuándo se testeó esto por última vez?» pasa a tener
// respuesta (I-5).
//
// Las tres piezas importan y por razones distintas:
//
//   - `from_cache` dice que no ejecutó;
//   - `reason` dice POR QUÉ, y sin él el booleano no distingue un caché que
//     funciona de una configuración que revive basura (spec 15 §5.4);
//   - `evidence_from` dice CUÁL registro lo revivió, entero — no una fecha
//     formateada dentro de una frase, que es lo único que había hasta ahora.
func TestHechos_UnStepQueReviveLoDiceYDiceContraQue(t *testing.T) {
	h := newHarness(t)

	// Hacen falta TRES corridas y no dos: cada step guarda lo que produjo, la
	// corrida siguiente lo carga antes de decidir y la huella de variables se
	// mueve una vez más (la divergencia viva que
	// `TestRunCommand_ReejecucionSinCambios` fija). A partir de la tercera el
	// punto es fijo, y es ahí donde hay un salto que observar.
	correHastaEstable(t, h)

	cerrados := deTipo(h.ultimaTira(), record.TypeStepFinished.String())
	require.Len(t, cerrados, 2)

	revivido := cerrados[0]
	assert.Equal(t, "01-test", revivido.Payload["step_id"])
	assert.Equal(t, command.StepCached.String(), revivido.Payload["status"],
		"revivir no es SUCCESS: sobre cero comandos, «se ejecutó correctamente» es una conclusión")
	assert.Equal(t, true, revivido.Payload["from_cache"])
	assert.Equal(t, command.ReasonUpToDate.String(), revivido.Payload["reason"])
	assert.Contains(t, revivido.Payload["step_fingerprint"], "ck-v1:",
		"la huella viaja aunque el salto no la escriba: se anota para informar, no para escribir")

	evidencia, ok := revivido.Payload["evidence_from"].(map[string]any)
	require.True(t, ok, "un salto sin evidencia deja la pregunta sin respuesta")
	assert.NotEmpty(t, evidencia["execution_id"])
	assert.NotEmpty(t, evidencia["at"])

	clave, ok := evidencia["state_key"].(map[string]any)
	require.True(t, ok, "la clave viaja por componentes, no como una cadena compuesta")
	assert.Equal(t, "01-test", clave["step_id"])
	assert.Equal(t, "environment:"+fixtureEnvironment, clave["scope"])

	// Y apunta al ÚLTIMO registro escrito para ese step, que es contra el que se
	// comparó. El archivo se llama como su `record_id` —es un ULID, así que el
	// orden lexicográfico es el temporal—, de modo que la referencia se puede
	// comprobar contra el disco sin volver a derivarla.
	registros := h.persistedStepState("01-test")
	require.NotEmpty(t, registros)
	ultimo := strings.TrimSuffix(filepath.Base(registros[len(registros)-1]), ".json")
	assert.Equal(t, ultimo, evidencia["record_id"],
		"la evidencia apunta al registro que de verdad revivió al step")
}

// LA MITAD SIMÉTRICA, que es la que la spec 28 necesita: un step que EJECUTÓ
// también dice qué registro dejó.
//
// Sin ella, anclar un rollback a una ejecución pasada exigiría derivar por
// fechas qué registro estuvo vigente en cada step, que es guardar una conclusión
// en vez de un hecho (28 §5.3).
func TestHechos_UnStepQueEjecutaTambienDiceQueRegistroDejo(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	cerrado := h.hechosDeTipo(record.TypeStepFinished.String())[0]
	require.Equal(t, command.StepSuccess.String(), cerrado.Payload["status"])

	evidencia, ok := cerrado.Payload["evidence_from"].(map[string]any)
	require.True(t, ok, "la evidencia viaja en las DOS mitades, no sólo en la del salto")

	registros := h.persistedStepState("01-test")
	require.Len(t, registros, 1)
	assert.Equal(t, strings.TrimSuffix(filepath.Base(registros[0]), ".json"),
		evidencia["record_id"], "es el registro que este step acaba de escribir")
}

// LOS SEIS MOTIVOS SON DATO, no una frase de log. Aquí se fijan los tres que el
// fixture puede producir sin inventarse un pipelinecode entero.
func TestHechos_ElMotivoViajaComoDato(t *testing.T) {
	t.Run("la primera vez: no consta registro", func(t *testing.T) {
		h := newHarness(t)
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		assert.Equal(t, command.ReasonNoRecord.String(),
			h.hechosDeTipo(record.TypeStepFinished.String())[0].Payload["reason"])
	})

	// EJECUTADO SIN PODER RECORDARSE (spec 13 §5.3): sin `config.yaml` no hay
	// ámbito, luego no hay dónde constar. Termina bien y no escribe registro, así
	// que un `step_finished` sin evidencia NO es una anomalía — es lo normal para
	// este step, y el hecho tiene que poder decirlo.
	t.Run("sin config.yaml: no hay dónde recordarlo", func(t *testing.T) {
		h := newHarness(t, withoutPipelineFile("steps/01-test/config.yaml"))
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		cerrado := h.hechosDeTipo(record.TypeStepFinished.String())[0]
		assert.Equal(t, command.ReasonNoScope.String(), cerrado.Payload["reason"])
		assert.Equal(t, command.StepSuccess.String(), cerrado.Payload["status"])
		assert.NotContains(t, cerrado.Payload, "evidence_from",
			"ejecuta, termina bien y no deja registro: la ausencia es el hecho")
		assert.Empty(t, cerrado.Payload["scope"], "sin ámbito declarado no hay ámbito que emitir")
	})

	// LA MISMA DECISIÓN POR LA OTRA MITAD (spec 15 §5.5): hay ámbito y no hay
	// ninguna afirmación que guardar.
	t.Run("sin rules: no hay nada que comprobar", func(t *testing.T) {
		h := newHarness(t, withPipelineFile("steps/01-test/config.yaml", "scope: project\n"))
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		cerrado := h.hechosDeTipo(record.TypeStepFinished.String())[0]
		assert.Equal(t, command.ReasonNoRules.String(), cerrado.Payload["reason"])
		assert.NotContains(t, cerrado.Payload, "evidence_from")
	})

	// UN STEP SIN COMANDOS es `SKIPPED` y no `SUCCESS` (spec 04 §5.3), y hasta la
	// spec 19 la ÚNICA diferencia observable entre los dos era la ausencia de su
	// registro en el almacén: había que inferir de un archivo que no está un hecho
	// que el motor ya calculaba.
	// Se vacía `02-supply` y no `01-test` porque el fixture los encadena: el
	// primero produce `artifact_name` y el segundo lo interpola, así que vaciar el
	// primero mediría un fallo de interpolación en vez de un salto.
	t.Run("sin comandos: saltado, no exitoso", func(t *testing.T) {
		h := newHarness(t, withPipelineFile("steps/02-supply/commands.yaml", ""))
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		cerrados := h.hechosDeTipo(record.TypeStepFinished.String())
		require.Len(t, cerrados, 2)
		cerrado := cerrados[1]
		assert.Equal(t, command.StepSkipped.String(), cerrado.Payload["status"])
		assert.Equal(t, command.ReasonNoCommands.String(), cerrado.Payload["reason"])
		assert.Equal(t, false, cerrado.Payload["from_cache"],
			"saltado y revivido son dos cosas distintas: uno no tenía qué ejecutar")
	})
}

// ── Los parámetros ──────────────────────────────────────────────────────────

// `parameter_resolved` ES POR PARÁMETRO (N-3), no un digest agregado, y lleva su
// procedencia como DATO.
//
// El valor NO entra: entra su resumen. Es la regla de la spec 14 —en la
// identidad entra la declaración, nunca el valor— aplicada al registro, porque
// un registro que viaja fuera de la organización no puede llevar secretos en
// claro.
func TestHechos_CadaParametroResueltoDejaElSuyo(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	parametros := h.hechosDeTipo(record.TypeParameterResolved.String())
	require.Len(t, parametros, 2, "las dos salidas con nombre del fixture")

	nombres := make([]string, 0, len(parametros))
	for _, parametro := range parametros {
		nombres = append(nombres, parametro.Payload["name"].(string))

		assert.Equal(t, command.OriginRuntime.String(), parametro.Payload["source"],
			"lo extraído del stdout es lo que el mundo real devolvió al ejecutar")

		digest, _ := parametro.Payload["digest"].(string)
		assert.True(t, strings.HasPrefix(digest, "sha256:"), "digest %q", digest)
	}
	assert.Equal(t, []string{"artifact_name", "acr_name"}, nombres)

	// La afirmación que la spec 20 relaja de forma controlada, y hasta entonces es
	// absoluta: NINGÚN hecho lleva el valor en claro ni salida de un comando.
	for _, parametro := range parametros {
		for _, valor := range parametro.Payload {
			texto, ok := valor.(string)
			if !ok {
				continue
			}
			assert.NotContains(t, texto, "demo-app",
				"el valor resuelto no puede aparecer en claro en ningún campo")
		}
	}
}

// SIN EXTRACTOS TODAVÍA: ningún hecho contiene stdout ni stderr. La aserción es
// explícita porque es la spec 20 quien la relaja, y hacerlo tiene que ser una
// decisión visible y no un descuido.
func TestHechos_NingunHechoLlevaLaSalidaDeUnComando(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/01-test/commands.yaml",
		"- name: build\n  cmd: echo SECRETO_EN_STDOUT; exit 5\n"))

	require.Equal(t, cli.ExitFailed, h.run().exitCode)

	for _, hecho := range h.hechos() {
		for _, valor := range hecho.Payload {
			texto, ok := valor.(string)
			if !ok {
				continue
			}
			assert.NotContains(t, texto, "SECRETO_EN_STDOUT",
				"el hecho '%s' lleva salida de comando", hecho.Type)
		}
	}
}

// ── El pliegue coincide con lo observado ────────────────────────────────────

// `Fold` RECONSTRUYE. **Es la verificación de que emisión y modelo encajan**: si
// el emisor deja un hueco o escribe un hecho que el pliegue no espera, el
// resultado derivado deja de parecerse a la ejecución que de verdad ocurrió.
//
// Se pliega lo que el motor ESCRIBIÓ, leído del JSONL, no eventos fabricados a
// mano: fabricarlos probaría que `Fold` sabe plegar, que ya lo prueba su propio
// test, y no que lo emitido sea plegable.
func TestHechos_FoldReconstruyeLaEjecucion(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	resultado := record.Fold(h.eventos())

	assert.Equal(t, record.AttemptSucceeded, resultado.Status)
	assert.True(t, resultado.IsTerminal())
	assert.Equal(t, h.cabezaDelLinaje(fixtureEnvironment), resultado.Deployment.String())
	assert.Equal(t, "02-supply", resultado.LastStep)
	assert.Equal(t, 2, resultado.Commands)
	assert.Zero(t, resultado.FailedCommands)
	assert.Zero(t, resultado.StaleClones)
	assert.Nil(t, resultado.ExitCode)

	require.Len(t, resultado.Steps, 2)
	supply, ok := resultado.Step("02-supply")
	require.True(t, ok)
	assert.True(t, supply.Finished)
	assert.Equal(t, command.StepSuccess, supply.Status)
	assert.Equal(t, command.ReasonNoRecord, supply.Reason)
	assert.Equal(t, "environment:"+fixtureEnvironment, supply.Scope.String(),
		"el ámbito llega como campo del hecho, no descomponiendo un hash")
	assert.Contains(t, supply.StepFingerprint, "ck-v1:")
	assert.False(t, supply.Evidence.IsZero())
}

// Y lo reconstruye igual cuando la ejecución FALLA: el exit code y la clase de
// error del intento son los del PRIMER step que falló, porque los posteriores no
// llegaron a correr.
func TestHechos_FoldReconstruyeUnFallo(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/01-test/commands.yaml",
		"- name: build\n  cmd: exit 3\n"))

	require.Equal(t, cli.ExitFailed, h.run().exitCode)

	resultado := record.Fold(h.eventos())

	assert.Equal(t, record.AttemptFailed, resultado.Status)
	assert.Equal(t, "01-test", resultado.LastStep)
	require.NotNil(t, resultado.ExitCode)
	assert.Equal(t, 3, *resultado.ExitCode)
	assert.Equal(t, record.ErrorClassCommandFailed, resultado.ErrorClass)
	assert.Equal(t, 1, resultado.FailedCommands)
}

// ── Cancelación e interrupción ──────────────────────────────────────────────

// UNA CANCELACIÓN SE ESCRIBE, y por eso se distingue de una interrupción.
//
// `SIGINT` cancela el contexto, el comando en curso muere y la cadena devuelve
// error — pero quien está vivo para escribir el desenlace no fue interrumpido, y
// una decisión no es una desgracia. Es la asimetría que `AttemptFinished`
// impone rechazando `interrupted` en su `Validate`.
func TestHechos_UnaCancelacionSeEscribeYSeDistingueDeUnaInterrupcion(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/01-test/commands.yaml",
		"- name: build\n  cmd: sleep 30\n"))

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	args := h.args()
	args.InputFile = h.writeRequest(h.request())
	result := h.executeCtx(ctx, args, nil)
	require.Equal(t, cli.ExitCancelled, result.exitCode, result.stderr)

	cerrados := h.hechosDeTipo(record.TypeAttemptFinished.String())
	require.Len(t, cerrados, 1)
	assert.Equal(t, record.AttemptCancelled.String(), cerrados[0].Payload["status"])

	resultado := record.Fold(h.eventos())
	assert.Equal(t, record.AttemptCancelled, resultado.Status)
	assert.NotEqual(t, record.AttemptInterrupted, resultado.Status)
	assert.Equal(t, "01-test", resultado.LastStep,
		"y dice hasta dónde se llegó, que es lo que hace útil un intento que no terminó")
}

// LA INTERRUPCIÓN SE DERIVA DE UNA AUSENCIA, y es el caso que justifica el
// modelo entero.
//
// No se puede matar el proceso de test, así que se monta la MISMA forma que deja
// una muerte dura: los hechos escritos hasta ahí, sin el desenlace. Que `Fold`
// produzca un resultado útil en vez de nada es lo que hace que una Fly Machine
// que se va a mitad deje datos aprovechables.
func TestHechos_SinDesenlaceElPliegueDiceInterrumpido(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	truncados := make([]record.Event, 0, 8)
	for _, evento := range h.eventos() {
		if evento.Type() == record.TypeAttemptFinished {
			continue
		}
		truncados = append(truncados, evento)
	}

	resultado := record.Fold(truncados)

	assert.Equal(t, record.AttemptInterrupted, resultado.Status)
	assert.False(t, resultado.IsTerminal())
	assert.Equal(t, "02-supply", resultado.LastStep)
	assert.Zero(t, resultado.Duration, "un intento interrumpido no tiene duración: tiene un principio")
}

// ── Utilidades ──────────────────────────────────────────────────────────────

func idsDeStep(hechos []jsonlEvento) []string {
	ids := make([]string, 0, len(hechos))
	for _, hecho := range hechos {
		ids = append(ids, hecho.Payload["step_id"].(string))
	}
	return ids
}
