package cli_test

// Volver a una ejecución exitosa pasada usa el estado de ENTONCES (spec 28 §7).
//
// # El fixture del rollback, y por qué es otro
//
// El pipelinecode por defecto del harness no sirve para observar un ancla, y la
// razón merece estar escrita porque es la misma que hace difícil el caso real:
// con `rules: [state_changed]` un step sólo re-ejecuta cuando su declaración o
// el proyecto cambian, y si nada cambia entre dos corridas **no hay estado nuevo
// del que distinguir el viejo**. Un ancla se ve cuando el almacén ha avanzado y
// la huella no, que es exactamente lo que `max_age` produce.
//
// Así que `02-supply` declara aquí `max_age` y nada más: se ejecuta en cada
// corrida, escribe un registro cada vez y NO compara contenidos —su registro
// sale sin huella, que es el mecanismo que la spec 15 §5.4 ya tenía—. Con eso,
// la única diferencia entre dos corridas es qué registro estuvo vigente.
//
// Lo que el step CONSUME es su propio registro anterior: `acr_previo` está
// declarado como literal en `variables/sand/supply.yaml` —o sea que en la
// primera corrida vale «ninguno»— y a partir de la segunda gana el valor del
// almacén, porque `OriginState` está por encima de `OriginDeclared` (spec 12).
// Es la infraestructura de §2 en miniatura: un valor que avanza por su cuenta
// entre despliegues.
//
// El valor que PRODUCE viene de `$VEX_TEST_TAG`, que el shell interpola y el
// motor no ve. Es lo que permite cambiar lo que una corrida deja escrito **sin
// tocar el pipelinecode ni el proyecto**, y por tanto sin mover el `content_id`
// ni ninguna huella: las cuatro corridas de un caso declaran exactamente la
// misma intención.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// supplyConMaxAge hace que `02-supply` se ejecute en todas las corridas.
//
// `max_age: 1ns` es «caducado siempre» sin serlo por definición: la regla existe,
// así que el step SÍ escribe registro —un step sin `rules` no escribe ninguno
// (spec 15 §5.5), y sin registro no hay a qué anclar—.
const supplyConMaxAge = "scope: environment\nrules:\n  - max_age: 1ns\n"

// supplyQueRecuerdaLoAnterior es el comando que hace visible el ancla: publica
// qué valor tenía en el almacén ANTES de correr, y deja uno nuevo.
const supplyQueRecuerdaLoAnterior = `- name: provision
  description: publica el registro que estuvo vigente y deja el suyo
  cmd: echo '02-supply previo = "${var.acr_previo}" acr_previo = "tag-'"$VEX_TEST_TAG"'"' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: acr_previo
      description: el registro aprovisionado en esta corrida
      probe: acr_previo = "([^"]+)"
`

// acrPrevioDeclarado es el literal del que sale el valor de la PRIMERA corrida.
// Es un default —`OriginDeclared` es el origen más bajo— así que en cuanto haya
// registro deja de ganar (spec 12 §5.2).
const acrPrevioDeclarado = "- name: acr_previo\n  value: ninguno\n"

func harnessDeRollback(t *testing.T) *harness {
	t.Helper()
	return newHarness(t,
		withPipelineFile("steps/02-supply/config.yaml", supplyConMaxAge),
		withPipelineFile("steps/02-supply/commands.yaml", supplyQueRecuerdaLoAnterior),
		withPipelineFile("variables/sand/supply.yaml", acrPrevioDeclarado),
	)
}

// runConTag ejecuta una vez con el tag dado, dejando el log limpio para poder
// mirar SÓLO lo que esta corrida hizo.
//
// El tag va por el entorno y no por el pipelinecode a propósito: `$VEX_TEST_TAG`
// lo interpola el shell y el motor no lo ve, así que cambiar lo que una corrida
// deja escrito no mueve ninguna huella ni el `content_id`.
func (h *harness) runConTag(t *testing.T, tag string, opts ...requestOption) runResult {
	t.Helper()
	t.Setenv("VEX_TEST_TAG", tag)
	h.resetLog()
	return h.run(opts...)
}

// corridaConTag ejecuta una vez con éxito y devuelve el `deployment_id`.
func corridaConTag(t *testing.T, h *harness, tag string, opts ...requestOption) string {
	t.Helper()
	result := h.runConTag(t, tag, opts...)
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	return h.elDespliegueDeLaUltimaCorrida()
}

// loQueVioSupply es el valor que `02-supply` leyó del almacén al empezar.
func loQueVioSupply(t *testing.T, h *harness) string {
	t.Helper()
	for _, linea := range h.logLines() {
		if !strings.HasPrefix(linea, "02-supply ") {
			continue
		}
		_, resto, _ := strings.Cut(linea, `previo = "`)
		valor, _, _ := strings.Cut(resto, `"`)
		return valor
	}
	t.Fatal("02-supply no dejó ninguna línea en el log")
	return ""
}

// EL CASO QUE DA NOMBRE A LA SPEC (§7, primer punto).
//
// Ejecutar E; ejecutar dos despliegues más que cambien el estado; volver a E ⇒
// los steps leen los registros DE E, no los últimos. Sin el ancla leerían los de
// la tercera ejecución, que es exactamente el rollback a medias de §1: la
// aplicación vuelve atrás y la infraestructura no.
func TestRunCommand_UnRollbackLeeElEstadoDeEntoncesYNoElUltimo(t *testing.T) {
	h := harnessDeRollback(t)

	deE := corridaConTag(t, h, "v1")
	assert.Equal(t, "ninguno", loQueVioSupply(t, h),
		"en la primera corrida no hay registro: gana el literal declarado")

	corridaConTag(t, h, "v2")
	assert.Equal(t, "tag-v1", loQueVioSupply(t, h))

	corridaConTag(t, h, "v3")
	assert.Equal(t, "tag-v2", loQueVioSupply(t, h),
		"el estado avanza por su cuenta entre despliegues: es el problema de §1")

	// Y ahora la vuelta atrás. Sin ancla, `02-supply` vería `tag-v3`.
	corridaConTag(t, h, "v4", withRollbackTo(deE, 1))

	assert.Equal(t, "tag-v1", loQueVioSupply(t, h),
		"el registro vigente en R es el que estuvo vigente en E, no el último")
}

// Y el matiz que hace que lo anterior signifique lo que dice: para un step que
// EJECUTÓ, el registro vigente en aquel intento es **el que aquel intento
// escribió**, no el que leyó al empezar.
//
// Es la mitad simétrica de `evidence_from` (spec 19), y es la semántica correcta:
// lo que un rollback tiene que reproducir es el estado que E DEJÓ —el ARN que E
// aprovisionó—, no el que E encontró. Anclar a lo que leyó devolvería el mundo un
// despliegue más atrás de lo que se pidió.
func TestRunCommand_ElRegistroAncladoEsElQueAquelIntentoEscribio(t *testing.T) {
	h := harnessDeRollback(t)

	deE := corridaConTag(t, h, "v1")
	assert.Equal(t, "ninguno", loQueVioSupply(t, h), "E LEYÓ el literal declarado")
	assert.Equal(t, map[string]string{"acr_previo": "tag-v1"},
		h.storedVars(fixtureEnvironment, "02-supply"), "y ESCRIBIÓ tag-v1")

	corridaConTag(t, h, "v2")
	corridaConTag(t, h, "v3", withRollbackTo(deE, 1))

	assert.Equal(t, "tag-v1", loQueVioSupply(t, h),
		"R parte de lo que E dejó, que es lo que un rollback tiene que reproducir")
}

// §5.1 y §7 (segundo punto): R tiene el MISMO `content_id` que E y OTRO
// `deployment_id`. Es el test que justifica que la spec 17 separara las dos
// identidades — con una sola, R y E serían indistinguibles o incomparables.
func TestRunCommand_UnRollbackRepiteElContenidoYCambiaDePosicion(t *testing.T) {
	h := harnessDeRollback(t)

	deE := corridaConTag(t, h, "v1")
	corridaConTag(t, h, "v2")
	deR := corridaConTag(t, h, "v3", withRollbackTo(deE, 1))

	assert.NotEqual(t, deE, deR, "R cae en otra posición de la historia: cuelga de lo vigente ahora")

	// Un solo objeto para las tres corridas: la intención es la misma, y la
	// tienda está direccionada por contenido. Es `content_id` idéntico dicho de la
	// forma más directa que hay.
	objetos := h.objetos()
	require.Len(t, objetos, 1,
		"las tres corridas declaran la misma intención, así que hay UN objeto")

	// Y la cabeza del linaje es la de R: un rollback es una ejecución más en la
	// historia del ambiente, no una rama.
	assert.Equal(t, deR, h.cabezaDelLinaje(fixtureEnvironment))
}

// §5.5 y §7 (último punto de la 21): «esto fue una vuelta atrás» es auditable
// desde FUERA de la máquina, porque viaja en un hecho que se empuja al destino.
//
// Va en el hecho y no en el objeto a propósito: meterlo en el objeto cambiaría el
// `content_id`, y que R y E lo compartan es la mitad del diseño.
func TestRunCommand_ElRollbackQuedaConfirmadoEnElRegistro(t *testing.T) {
	h := harnessDeRollback(t)

	deE := corridaConTag(t, h, "v1")
	abierto := deTipo(h.ultimaTira(), record.TypeAttemptStarted.String())[0]
	assert.NotContains(t, abierto.Payload, "rollback_to",
		"una ejecución normal no afirma que vuelve a ninguna parte")

	corridaConTag(t, h, "v2", withRollbackTo(deE, 1))

	abierto = deTipo(h.ultimaTira(), record.TypeAttemptStarted.String())[0]
	confirmado, ok := abierto.Payload["rollback_to"].(map[string]any)
	require.True(t, ok, "el motor confirma en el registro que entendió el ancla")
	assert.Equal(t, deE, confirmado["deployment_id"])
	assert.EqualValues(t, 1, confirmado["attempt"])

	// Y llega al destino: el área de trabajo muere con la máquina efímera.
	enDestino := deTipo(h.hechosDelDestino(), record.TypeAttemptStarted.String())
	conRollback := 0
	for _, hecho := range enDestino {
		if _, tiene := hecho.Payload["rollback_to"]; tiene {
			conRollback++
		}
	}
	assert.Equal(t, 1, conRollback, "una de las dos corridas fue una vuelta atrás")
}

// §5.3 y §7 (tercer punto): un rollback NO relaja ninguna comprobación.
//
// Si entre E y R cambió el `commands.yaml` de un step, ese step **se ejecuta** en
// R aunque su registro anclado exista. Y el motor lo dice: el contenido de hoy no
// es el de entonces, así que esto se parece a un rollback y no lo es del todo.
func TestRunCommand_UnRollbackNoEsUnPermisoParaSaltarseComprobaciones(t *testing.T) {
	h := harnessDeRollback(t)

	deE := corridaConTag(t, h, "v1")
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

	// Sin tocar nada, `01-test` revive: su declaración y el proyecto son los
	// mismos.
	corridaConTag(t, h, "v2")
	assert.Equal(t, []string{"02-supply"}, h.ranSteps(),
		"01-test revive: nada de lo que declara cambió")

	// Ahora cambia su `commands.yaml` y se vuelve, anclando a E.
	writeFile(t, filepath.Join(h.pipelineDir, "steps", "01-test", "commands.yaml"),
		`- name: build
  cmd: echo "01-test BUILD SUCCESS artifact=demo-app-editado" | tee -a "$VEX_TEST_LOG"
  outputs:
    - probe: BUILD SUCCESS
    - name: artifact_name
      probe: artifact=([a-z0-9-]+)
`)
	commitAll(t, h.pipelineDir, "feat: cambia lo que 01-test declara")

	args, lineas := h.capturarLineas()
	t.Setenv("VEX_TEST_TAG", "v3")
	h.resetLog()
	args.InputFile = h.writeRequest(h.request(withRollbackTo(deE, 1)))
	result := h.execute(args, nil)
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	assert.Contains(t, h.ranSteps(), "01-test",
		"su huella cambió respecto del registro anclado, así que se ejecuta")
	assert.Contains(t, lineas(), "no es el de",
		"y el motor avisa de que el contenido de hoy no es el de entonces")

	// Dos objetos: la intención cambió, así que R no es estrictamente E.
	assert.Len(t, h.objetos(), 2)
}

// §5.2 y §7 (cuarto punto): un destino inválido se rechaza ANTES del primer
// step, nunca a mitad.
func TestRunCommand_UnDestinoDeRollbackInvalidoSeRechazaAntesDeEmpezar(t *testing.T) {
	t.Run("no consta ese despliegue", func(t *testing.T) {
		h := harnessDeRollback(t)
		corridaConTag(t, h, "v1")

		h.resetLog()
		result := h.runConTag(t, "v2",
			withRollbackTo("dep-v1:"+strings.Repeat("a", 64), 1))

		require.Equal(t, cli.ExitFailed, result.exitCode)
		assert.Contains(t, result.stderr, "no consta")
		assert.Empty(t, h.ranSteps(), "ningún step llegó a correr")
	})

	t.Run("ese intento no existe", func(t *testing.T) {
		h := harnessDeRollback(t)
		deE := corridaConTag(t, h, "v1")

		h.resetLog()
		result := h.runConTag(t, "v2", withRollbackTo(deE, 7))

		require.Equal(t, cli.ExitFailed, result.exitCode)
		assert.Contains(t, result.stderr, "no consta")
		assert.Empty(t, h.ranSteps())
	})

	t.Run("el intento falló", func(t *testing.T) {
		h := newHarness(t,
			withPipelineFile("steps/02-supply/commands.yaml",
				"- name: provision\n  cmd: exit 3\n"))

		result := h.run()
		require.Equal(t, cli.ExitFailed, result.exitCode)
		fallido := h.elDespliegueDeLaUltimaCorrida()

		h.resetLog()
		result = h.run(withRollbackTo(fallido, 1))

		require.Equal(t, cli.ExitFailed, result.exitCode)
		assert.Contains(t, result.stderr, "no es un destino válido")
		assert.Contains(t, result.stderr, "02-supply", "y dice qué step lo descalifica")
	})
}

// §5.5 y §7 (último punto): un `rollback_to` MAL FORMADO es un error de
// invocación —exit code 2— y no un fallo de pipeline.
//
// Es lo que impide el modo de fallo que la spec 16 anota: un par que no compone
// produciría un ancla vacía, o sea un despliegue normal donde el usuario pidió
// volver atrás.
func TestRunCommand_UnRollbackMalFormadoEsErrorDeInvocacion(t *testing.T) {
	casos := []struct {
		nombre  string
		id      string
		intento int
		dice    string
	}{
		{"identificador sin forma", "no-es-un-despliegue", 1, "rollback_to"},
		{"el intento 0 no es un intento", "dep-v1:" + strings.Repeat("a", 64), 0, "intento"},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			h := harnessDeRollback(t)

			result := h.runConTag(t, "v1", withRollbackTo(caso.id, caso.intento))

			assert.Equal(t, cli.ExitInputError, result.exitCode,
				"el contrato de entrada se valida en el borde")
			assert.Contains(t, result.stderr, caso.dice)
			assert.Empty(t, h.ranSteps())
		})
	}
}

// §5.2 y §7 (quinto punto): un step que REVIVIÓ en E es anclable. R usa el
// registro que E revivió.
//
// Es la mitad del criterio que es fácil de escribir al revés: excluir los steps
// revividos haría que una ejecución fuera peor destino de rollback cuanto mejor
// funcionó el motor.
func TestRunCommand_UnStepRevividoEnElDestinoEsAnclable(t *testing.T) {
	h := harnessDeRollback(t)

	corridaConTag(t, h, "v1")
	require.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

	// En la segunda corrida `01-test` REVIVE, así que su `evidence_from` apunta al
	// registro que escribió la primera. Anclar aquí es anclar a ese registro.
	deE := corridaConTag(t, h, "v2")
	require.Equal(t, []string{"02-supply"}, h.ranSteps())
	registroDeE := registroRevividoPor(t, h, "01-test")

	corridaConTag(t, h, "v3")
	corridaConTag(t, h, "v4", withRollbackTo(deE, 1))

	assert.Equal(t, registroDeE, registroRevividoPor(t, h, "01-test"),
		"R revive con el MISMO registro con el que revivió el intento anclado")
	assert.Equal(t, "tag-v2", loQueVioSupply(t, h),
		"y el step que sí ejecutó en E se ancla al registro que E dejó")
}

// registroRevividoPor es el `record_id` con el que un step revivió en la última
// corrida. Falla si no revivió: un caso que mide el ancla de un step revivido
// tiene que ver primero que revivió.
func registroRevividoPor(t *testing.T, h *harness, stepID string) string {
	t.Helper()
	for _, hecho := range deTipo(h.ultimaTira(), record.TypeStepFinished.String()) {
		if texto(hecho.Payload, "step_id") != stepID || hecho.Payload["from_cache"] != true {
			continue
		}
		evidencia, ok := hecho.Payload["evidence_from"].(map[string]any)
		require.True(t, ok, "un step revivido dice QUÉ registro lo revivió, y eso es lo anclable")
		return texto(evidencia, "record_id")
	}
	t.Fatalf("%s no revivió en la última corrida", stepID)
	return ""
}

// §5.2 y §7 (sexto punto): los steps que no persisten no rompen el ancla. R los
// ejecuta, igual que hizo E.
//
// `01-test` se queda sin `config.yaml`: no declara ámbito, luego no hay dónde
// recordarse, luego se ejecuta siempre y no escribe registro (spec 13 §5.3). Su
// `step_finished` sale SIN `evidence_from`, y el ancla tiene que tolerarlo en vez
// de rechazar el destino.
func TestRunCommand_UnStepQueNoPersisteNoRompeElAncla(t *testing.T) {
	h := newHarness(t,
		withoutPipelineFile("steps/01-test/config.yaml"),
		withPipelineFile("steps/02-supply/config.yaml", supplyConMaxAge),
		withPipelineFile("steps/02-supply/commands.yaml", supplyQueRecuerdaLoAnterior),
		withPipelineFile("variables/sand/supply.yaml", acrPrevioDeclarado),
	)

	deE := corridaConTag(t, h, "v1")
	corridaConTag(t, h, "v2")
	corridaConTag(t, h, "v3", withRollbackTo(deE, 1))

	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps(),
		"un step sin registro se ejecuta en el rollback igual que en el destino")
	assert.Equal(t, "tag-v1", loQueVioSupply(t, h),
		"y el que sí lo tiene se ancla igual")
}

// §5.4 y §7 (séptimo y octavo puntos): el ancla es INMUTABLE y después de R no
// hay modo especial del que salir.
//
// Lo que R escribe no cambia de qué registro parten los steps que le quedan —se
// observa en que la corrida SIGUIENTE parte de lo que R dejó, por la vía normal
// del último registro—.
func TestRunCommand_DespuesDeUnRollbackNoHayModoEspecial(t *testing.T) {
	h := harnessDeRollback(t)

	deE := corridaConTag(t, h, "v1")
	corridaConTag(t, h, "v2")
	corridaConTag(t, h, "v3", withRollbackTo(deE, 1))
	require.Equal(t, "tag-v1", loQueVioSupply(t, h))

	// Sin `rollback_to`: la ejecución siguiente parte del registro de R.
	corridaConTag(t, h, "v4")

	assert.Equal(t, "tag-v3", loQueVioSupply(t, h),
		"lo que R escribió es el último registro, y la corrida siguiente lo lee sin más")
	assert.Equal(t, map[string]string{"acr_previo": "tag-v4"},
		h.storedVars(fixtureEnvironment, "02-supply"))
}

// La vuelta atrás se LEE del registro: `record show` y `record history` la
// dicen, y ninguna de las dos consulta el `RequestInput` que la pidió.
//
// Es lo que cierra la objeción de la spec 16 a no subir `schema_version`: el
// cliente puede comprobar que el motor entendió el ancla en vez de confiar en
// que un campo opcional no se descartó en silencio.
func TestRecord_LaVueltaAtrasSeLeeDelRegistro(t *testing.T) {
	h := harnessDeRollback(t)

	deE := corridaConTag(t, h, "v1")
	deR := corridaConTag(t, h, "v2", withRollbackTo(deE, 1))

	assert.NotContains(t, h.recordShow(deE), "vuelta atrás",
		"el intento anclado no volvió a ninguna parte")
	assert.Contains(t, h.recordShow(deR), "vuelta atrás a: "+deE)

	historia := h.recordHistory(fixtureEnvironment)
	assert.Contains(t, historia, "↩ vuelta atrás a "+deE)
	assert.Equal(t, 1, strings.Count(historia, "↩ "),
		"uno de los dos intentos fue un rollback")

	// Y el pliegue lo trae como dato, no como texto: es lo que un consumidor de
	// otra plataforma va a leer (spec 26).
	assert.True(t, h.recordAttempt(deR, 1).RollbackTo.Deployment().Equals(
		h.recordAttempt(deE, 1).Deployment))
}

// §5.2, tercera condición (recuadro de la spec 24): el destino tiene que
// declarar la MISMA lista de steps que la operación que se va a ejecutar.
//
// Sin esta comprobación, volver a un intento de otra generación del pipelinecode
// produciría un rollback que no ancla ni un solo step y no lo dice, que es el
// modo de fallo silencioso que §4 rechaza.
func TestRunCommand_UnDestinoConOtraListaDeStepsSeRechazaNombrandoLaDiferencia(t *testing.T) {
	h := harnessDeRollback(t)

	// `test` es la operación de UN step; `supply` la de dos.
	deTest := corridaConTag(t, h, "v1", withStep("test"))

	h.resetLog()
	result := h.runConTag(t, "v2", withRollbackTo(deTest, 1))

	require.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "no declara los mismos steps")
	assert.Contains(t, result.stderr, "02-supply", "nombra la que sobra")
	assert.Empty(t, h.ranSteps(), "se rechaza antes del primer step")
}
