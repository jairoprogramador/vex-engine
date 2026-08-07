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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// correHastaEstable ejecuta hasta que ningún paso corre. Hoy hacen falta DOS
// ejecuciones: ver TestRunCommand_ReejecucionSinCambios.
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
	stored := h.storedVars("sand", "02-supply")
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

	// LOS DOS, y aquí se ve la regresión de eficiencia que la spec 10 §5.3
	// acepta a cambio de correctitud. Antes `test` tenía la regla de código y
	// `supply` no, así que este cambio re-ejecutaba uno y dejaba el otro
	// saltado. Con una clave única, TODO entra en la clave de todos los pasos:
	// `supply` se re-ejecuta ante un cambio de código que no le afecta.
	//
	// Se acepta porque la alternativa era peor: la selección por paso estaba
	// cableada en el motor POR NOMBRE, que es justo lo que P1 deroga. La spec 15
	// devuelve la granularidad, declarada por el pipeline en vez de cableada.
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

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

	// Los dos pasos, no sólo `test`: desde la spec 10 la huella del código entra
	// en la clave de todos. Lo que este caso fija es que el permiso SIGUE
	// moviendo la identidad al unificarse las claves; el caso es herencia de la
	// spec 08 y tenía que sobrevivir, no reescribirse desde cero.
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.Empty(t, h.ranSteps(), "el permiso ya está en la clave de la entrada escrita")
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

	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
}

// ── La clave de caché (spec 10) ─────────────────────────────────────────────

// La clave que se persiste y se compara lleva el prefijo de su regla de
// composición. Sin él, cambiar cómo se compone la clave sería indistinguible de
// un bug.
//
// Sustituye a `TestRunCommand_LaHuellaPersistidaLlevaLaVersionDeLaRegla`, que
// leía la huella de código del `.status` que la policy escribía: ese archivo ya
// no existe, y lo que se persiste ahora es la clave. La propiedad que aquel test
// defendía —que el prefijo de versión no se pierde por el camino— vive donde
// importa, porque la clave se compone sobre las formas canónicas COMPLETAS de
// las tres huellas.
func TestRunCommand_LaClavePersistidaLlevaLaVersionDeLaRegla(t *testing.T) {
	h := newHarness(t)
	correHastaEstable(t, h)

	claves := h.cacheEntries()

	require.NotEmpty(t, claves)
	for _, clave := range claves {
		assert.Regexp(t, `^ck-v1:[0-9a-f]{64}$`, clave)
	}
}

// LA REGRESIÓN DECLARADA de la spec 11 §5.5, y este test afirmaba lo CONTRARIO
// hasta la spec 10 — el diff conviene mirarlo.
//
// La spec 10 compró que `A → B → A` acertara: la entrada estaba direccionada por
// contenido, así que la de A seguía en su sitio cuando se escribía la de B. Con
// la clave de POSICIÓN, la decisión compara contra el ÚLTIMO registro, y el
// último es el de B: volver a A re-ejecuta.
//
// Se acepta porque re-ejecutar de más nunca produce un despliegue que no
// ocurrió, mientras que saltar de menos sí. Y lo que la haría barata de revertir
// sigue escrito en disco: el índice conserva la entrada de A —lo que este test
// comprueba en su segunda mitad—, así que la pregunta «¿existe algún registro
// con esta huella?» ya está respondida el día que se decida volver.
func TestRunCommand_VolverAUnEstadoYaEjecutadoReejecuta(t *testing.T) {
	h := newHarness(t)

	// Estado A.
	correHastaEstable(t, h)
	entradasEnA := h.cacheEntries()
	require.NotEmpty(t, entradasEnA)

	// Estado B: un byte distinto en el proyecto.
	h.writeProjectFile("src/app.txt", "v2\n")
	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.NotEmpty(t, h.ranSteps(), "el cambio a B re-ejecuta")

	// Vuelta a A, byte a byte.
	h.writeProjectFile("src/app.txt", "v1\n")
	h.resetLog()
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	assert.NotEmpty(t, h.ranSteps(),
		"la comparación es contra el ÚLTIMO registro, que es el de B")
	assert.Subset(t, h.cacheEntries(), entradasEnA,
		"pero el índice conserva la entrada de A: la regresión es barata de revertir")
}

// ── El estado append-only (spec 11) ─────────────────────────────────────────

// LA PRUEBA QUE DA NOMBRE A LA SPEC: ejecutar, cambiar algo, volver a ejecutar,
// y comprobar que LOS DOS REGISTROS existen. Hasta aquí el primero ya no estaba,
// y con él se iba el valor de ayer — que es a lo que un rollback tiene que poder
// apuntar (spec 28).
func TestRunCommand_ElRegistroDeAyerSigueAhi(t *testing.T) {
	h := newHarness(t)

	correHastaEstable(t, h)
	registrosTrasLaPrimera := h.persistedStepState("02-supply")
	require.NotEmpty(t, registrosTrasLaPrimera)

	h.writeProjectFile("src/app.txt", "v2\n")
	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	require.Contains(t, h.ranSteps(), "02-supply")

	registrosTrasLaSegunda := h.persistedStepState("02-supply")
	assert.Greater(t, len(registrosTrasLaSegunda), len(registrosTrasLaPrimera),
		"el contador de registros de una clave sólo sube")
	assert.Subset(t, registrosTrasLaSegunda, registrosTrasLaPrimera,
		"y los de ayer siguen en su sitio, con su nombre")
}

// §5.5.1 hecha ejecutable: **borrar el índice no cambia una sola decisión**, y
// el estado sobrevive al borrado.
//
// Es la comprobación que la versión anterior de esta spec no podía montar,
// porque no había dos tiendas con reglas de vida distintas.
func TestRunCommand_BorrarElIndiceNoCambiaNingunaDecision(t *testing.T) {
	h := newHarness(t)
	correHastaEstable(t, h)

	// El valor extraído del stdout —el que en un pipeline real es un ARN— está
	// en el almacén de registros, no en el índice.
	require.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])

	h.borrarElIndice()
	require.Empty(t, h.cacheEntries())

	h.resetLog()
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	assert.Empty(t, h.ranSteps(),
		"las mismas decisiones de salto: la decisión lee el registro, no el índice")
	assert.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"],
		"y el estado sobrevive: un ARN no vive en una tienda que alguien va a borrar")
}

// Aislamiento por ámbito: lo que un step deja en el ámbito de un AMBIENTE no se
// lee desde otro, y el ámbito de PROYECTO existe aparte, sin ambiente en su
// clave.
//
// El aislamiento es el mismo que la spec 10 compró metiendo el ambiente en el
// hash. Lo que cambia es de dónde sale: ahora viaja en la clave de posición, así
// que no puede caerse de ningún hash por descuido.
func TestRunCommand_ElEstadoSeAislaPorAmbito(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run(withEnvironment("sand")).exitCode)
	require.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])

	assert.Empty(t, h.storedVars("prod", "02-supply"),
		"lo del ámbito de un ambiente no se lee desde otro")
	assert.Empty(t, h.sharedVars("02-supply"),
		"y el ámbito de proyecto es otra clave: el fixture no produce variables compartidas")

	// El registro del ámbito de proyecto se escribe igualmente —el hecho que
	// guarda es «este step corrió», no «este step produjo algo»— y vive bajo
	// `project/`, sin ambiente en la ruta.
	registros := strings.Join(h.persistedStepState("02-supply"), "\n")
	assert.Contains(t, registros, filepath.Join("project", "02-supply"))
	assert.Contains(t, registros, filepath.Join("environment", "sand", "02-supply"))
}

// DOS PIPELINES, UN PROYECTO: comparten clave de estado y NO se reviven entre
// sí. Es el test que fija el argumento con el que D-A14 se cerró al revés
// (spec 11 §5.2).
//
// Compartir clave es lo que se quiere: el ACR pertenece al proyecto, así que un
// proyecto que cambia de plantilla no debe perder de vista los recursos que ya
// creó. Lo que impide que uno lea el trabajo del otro no es la clave sino la
// huella, que incluye el pipelinecode entero.
func TestRunCommand_DosPipelinesCompartenClaveYNoSeRevivenEntreSi(t *testing.T) {
	primero := newHarness(t)
	correHastaEstable(t, primero)
	registrosDelPrimero := primero.persistedStepState("02-supply")
	require.NotEmpty(t, registrosDelPrimero)

	segundo := primero.conOtroPipeline()

	segundo.resetLog()
	result := segundo.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, []string{"01-test", "02-supply"}, segundo.ranSteps(),
		"el pipelinecode entra en la huella: el segundo no revive el registro del primero")

	registrosDeLosDos := segundo.persistedStepState("02-supply")
	assert.Subset(t, registrosDeLosDos, registrosDelPrimero,
		"y los del primero siguen ahí: nadie sobrescribe a nadie")
	assert.Greater(t, len(registrosDeLosDos), len(registrosDelPrimero),
		"los dos escriben bajo la MISMA clave, que es lo que la spec decide")
}

// La clave no depende de la máquina, y por eso el caché compartido de la spec 16
// puede significar algo: el mismo árbol, el mismo proyecto y el mismo
// pipelinecode dan la misma clave desde otro $HOME y otra ruta absoluta.
//
// Es también donde muerde el defecto heredado de `.git` como archivo (spec 08
// §9.10, `cache/SPEC-v1.md` §3.3): en un worktree o un submódulo, `.git` entra
// en la huella con una ruta absoluta dentro, y este test se pondría en rojo. El
// fixture usa repos normales, así que hoy pasa; cuando el caché se comparta de
// verdad, este es el caso que hay que volver a mirar.
func TestRunCommand_LaClaveNoDependeDeLaMaquina(t *testing.T) {
	unaMaquina := newHarness(t)
	correHastaEstable(t, unaMaquina)

	// La otra máquina arranca con el caché FRÍO, así que ejecuta todo: lo que se
	// compara no es si se saltó, sino bajo QUÉ CLAVES quedaron sus entradas.
	otraMaquina := unaMaquina.otraMaquina()
	correHastaEstable(t, otraMaquina)

	assert.Equal(t, unaMaquina.cacheEntries(), otraMaquina.cacheEntries(),
		"dos raíces distintas con el mismo árbol dan las mismas claves")
}

// `show` no cambia qué se ejecuta, sólo si la salida se imprime — y entra en la
// huella igualmente (spec 10 §5.1bis). Sin esto, la secuencia es: un comando
// hace algo raro, el autor añade `show: true` para verlo, el step revive, no se
// imprime nada, y el autor concluye que `show` no funciona.
//
// El test de la spec 00 que afirmaba lo contrario —`el flag show NO entra en la
// huella`, en rules_test.go— se borró con el paquete entero. Lo que lo sustituye
// a nivel de unidad es `TestComputeInstructions_ShowEntraEnElMaterial`.
func TestRunCommand_AnadirShowReejecutaElStep(t *testing.T) {
	const supplyCmd = "steps/02-supply/commands.yaml"
	const sinShow = `
- name: provision
  cmd: echo '02-supply acr_name = "${var.registry_prefix}-${var.artifact_name}"' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: acr_name
      probe: acr_name = "([^"]+)"
`

	h := newHarness(t, withPipelineFile(supplyCmd, sinShow))
	correHastaEstable(t, h)

	// El ÚNICO cambio es `show: true`. Ni el comando, ni el workdir, ni los
	// outputs, ni el código, ni las variables.
	h.commitPipelineFile(supplyCmd, sinShow+"  show: true\n")

	h.resetLog()
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	assert.Equal(t, []string{"02-supply"}, h.ranSteps(),
		"añadir `show: true` para depurar tiene que volver a ejecutar el paso")
}

// Revivir no escribe NADA: ni registro ni entrada de índice.
//
// Son dos propiedades en una. La primera es la idempotencia de la spec 09 §9.10:
// si consultar escribiera, una muerte dura entre la consulta y el final del step
// dejaría grabado «ya se hizo» para un step que nunca terminó. La segunda es de
// la spec 11 §5.3 —«un registro por ejecución REAL, nunca cuando se revive»— y
// no es contable: un registro escrito al revivir saldría sin huella, y el step
// dejaría de revivir para siempre. Su síntoma sería que esta misma prueba
// oscilara entre ejecutar y revivir en corridas alternas.
func TestRunCommand_RevivirNoEscribeNada(t *testing.T) {
	h := newHarness(t)
	correHastaEstable(t, h)

	entradas := h.cacheEntries()
	registros := h.persistedStepState("02-supply")
	require.NotEmpty(t, entradas)
	require.NotEmpty(t, registros)

	// Tres corridas que no ejecutan nada: sólo consultan.
	for range 3 {
		h.resetLog()
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
		require.Empty(t, h.ranSteps())
	}

	assert.Equal(t, entradas, h.cacheEntries(),
		"consultar no añadió ni cambió ninguna entrada de índice")
	assert.Equal(t, registros, h.persistedStepState("02-supply"),
		"ni un registro: revivir no es un hecho nuevo del step")
}

// ── Aislamiento de ambiente (regresión de R-22) ─────────────────────────────

func TestRunCommand_AmbientesAislados(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run(withEnvironment("sand")).exitCode)
	assert.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])

	h.resetLog()
	result := h.run(withEnvironment("prod"))
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	// R-22: `supply@prod` no se salta pese a que `supply@sand` acaba de correr.
	//
	// Hasta la spec 10 pasaba por ACCIDENTE: la clave de las instrucciones y la
	// del código no llevaban el ambiente, y que `prod` no acertara con lo escrito
	// por `sand` dependía únicamente de que la variable `environment` estuviera en
	// el mapa acumulado y no figurara en la lista de exclusiones de la huella de
	// variables. Desde la 10 el ambiente está en la clave por derecho propio, en
	// `Scope`, y el caso que lo fija sin depender del material de variables es
	// `TestNewCacheKey_ElAmbienteNoSePuedeCaerDeLaClave`.
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	assert.Equal(t, "vexprod-demo-app", h.storedVars("prod", "02-supply")["acr_name"])

	// Los dos almacenes conviven: prod no pisó a sand.
	assert.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])
}

// ── Precedencia de variables (spec 12) ──────────────────────────────────────

// EL caso que da nombre a la spec 12: un literal declarado es un valor por
// DEFECTO, y en cuanto un step produce un valor para ese nombre el producido
// manda para el resto de la ejecución.
//
// Hasta la spec 12 ganaba el literal, porque el handler 03 escribía después que
// el 05 del step anterior sobre un mapa donde «el último gana». Es decir: el
// pipeline no podía reaccionar a lo que él mismo producía.
func TestRunCommand_LoProducidoEnEjecucionPisaAlLiteral(t *testing.T) {
	h := newHarness(t, withPipelineFile("variables/sand/supply.yaml",
		"- name: registry_prefix\n  value: vexsand\n- name: artifact_name\n  value: literal\n"))

	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	// 01-test produce artifact_name=demo-app por `outputs`; 02-supply lo declara
	// como literal en su variables/sand/supply.yaml. Gana el producido.
	assert.Equal(t, `02-supply acr_name = "vexsand-demo-app"`, h.logLines()[1])
	assert.Equal(t, "demo-app", h.storedVars("sand", "02-supply")["artifact_name"])
}

// El default sigue siendo un default: si nada produce el nombre, vale el
// literal. Es la otra mitad de la regla, y sin ella «lo declarado es un valor
// por defecto» sería «lo declarado no sirve para nada».
func TestRunCommand_ElLiteralValeCuandoNadieProduceEseNombre(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/01-test/commands.yaml", `
- name: build
  cmd: echo "01-test BUILD SUCCESS" | tee -a "$VEX_TEST_LOG"
  outputs:
    - probe: BUILD SUCCESS
`),
		withPipelineFile("variables/sand/supply.yaml",
			"- name: registry_prefix\n  value: vexsand\n- name: artifact_name\n  value: literal\n"))

	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, `02-supply acr_name = "vexsand-literal"`, h.logLines()[1])
}

// Dos comandos del mismo step que producen la misma variable: gana el SEGUNDO.
//
// Es la igualdad de la spec 12 §5.2 —«precedencia mayor o IGUAL»— y no es un
// detalle: sin ella un step no podría refinar un valor que él mismo acaba de
// extraer, que es el comportamiento de hoy y hay que conservarlo.
func TestRunCommand_DosComandosDelMismoStepElSegundoGana(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/02-supply/commands.yaml", `
- name: provision
  cmd: echo '02-supply acr_name = "primero"' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: acr_name
      probe: acr_name = "([^"]+)"
- name: refina
  cmd: echo '02-supply acr_name = "segundo"' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: acr_name
      probe: acr_name = "([^"]+)"
`))

	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, "segundo", h.storedVars("sand", "02-supply")["acr_name"])
}

// El almacén NO pisa lo que el motor inyecta en ESTA ejecución.
//
// `project_name` viene del RequestInput y lo pone el handler 07. Si una corrida
// anterior guardó otro valor bajo ese nombre, hasta la spec 12 el almacén —que
// cargaba después— lo sobrescribía: un dato de ayer pisando un hecho de hoy. Lo
// único que lo evitaba era que las volátiles no se persisten, y `project_name`
// no es volátil.
func TestRunCommand_ElAlmacenNoPisaLoInyectadoPorElMotor(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/02-supply/commands.yaml", `
- name: provision
  cmd: echo '02-supply project_name = "impostor"' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: project_name
      probe: project_name = "([^"]+)"
`))

	// Primera corrida: el step produce project_name=impostor y lo guarda. Dentro
	// de esa misma corrida gana el producido, que es la regla de arriba.
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	require.Equal(t, "impostor", h.storedVars("sand", "02-supply")["project_name"])

	// El step deja de producirlo: ahora solo lo consume. El impostor sigue en el
	// almacén, y es lo ÚNICO que aporta ese nombre aparte del handler 07.
	h.commitPipelineFile("steps/02-supply/commands.yaml",
		"- name: comprueba\n  cmd: echo '02-supply project_name=${var.project_name}' | tee -a \"$VEX_TEST_LOG\"\n")

	h.resetLog()
	result = h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	// Hasta la spec 12 el almacén cargaba DESPUÉS del handler 07 y esto decía
	// `impostor`: un dato de ayer pisando un hecho de hoy.
	assert.Contains(t, h.logLines(), "02-supply project_name=demo-app",
		"un valor de una corrida anterior no puede pisar un hecho de ésta")
	assert.Equal(t, "demo-app", h.storedVars("sand", "02-supply")["project_name"])
}

// El canario de que el orden de `chainStepHandlers` dejó de ser normativo
// (spec 12 §5.2' y §7) NO vive aquí: construir la cadena en el orden viejo
// exigiría un punto de extensión en `BuildRunCommand` que solo usarían los
// tests. Vive donde la cadena se puede armar de las dos formas sin tocar el
// cableado de producción, con los handlers 01, 02 y 03 REALES:
// `TestVarsChain_ElOrdenDeLosHandlersYaNoDecideQuienGana`
// (internal/domain/step), más las 24 permutaciones de
// `TestExecutionVariableMap_Add_ElOrdenDeLlegadaNoCambiaElResultado`
// (internal/domain/command).

// La otra cara del canario: el orden de la cadena dejó de decidir QUIÉN GANA,
// pero no es indiferente.
//
// El handler 03 no solo añade variables declaradas: las RESUELVE, interpolando
// `${var.…}` contra el mapa acumulado tal como esté en ese instante. Si carga
// antes que los handlers del almacén, el registro del PROPIO step queda fuera de
// su vista y un literal que lo interpole falla con «variable faltante» — la
// ejecución entera, no solo ese valor.
//
// Es el hallazgo que retira el reordenamiento que pedía la spec 12 §5.3 (ver su
// §10, H1): con la precedencia en `Add` el resultado es el mismo en los dos
// órdenes, así que mover el 03 no compraba nada y costaba esto.
func TestRunCommand_UnLiteralPuedeInterpolarElRegistroDelPropioStep(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/02-supply/commands.yaml", `
- name: provision
  cmd: echo '02-supply arn = "arn:aws:demo"' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: arn_propio
      probe: arn = "([^"]+)"
`))

	// Corrida 1: 02-supply produce `arn_propio` y lo guarda en SU registro. Nadie
	// más aporta ese nombre.
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	require.Equal(t, "arn:aws:demo", h.storedVars("sand", "02-supply")["arn_propio"])

	// Corrida 2: el step deja de producirlo y un literal declarado lo interpola.
	// La única fuente de `arn_propio` es ahora el registro del propio step.
	h.commitPipelineFile("variables/sand/supply.yaml",
		"- name: registry_prefix\n  value: vexsand\n- name: derivada\n  value: \"${var.arn_propio}/x\"\n")
	h.commitPipelineFile("steps/02-supply/commands.yaml",
		"- name: usa\n  cmd: echo '02-supply derivada=${var.derivada}' | tee -a \"$VEX_TEST_LOG\"\n")

	h.resetLog()
	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Contains(t, h.logLines(), "02-supply derivada=arn:aws:demo/x")
}

// La precedencia entre el almacén y lo declarado, invertida por la spec 12.
//
// Hasta la 12 ganaba lo declarado y este test se llamaba
// `TestRunCommand_PrecedenciaDeclaradoSobreAlmacenado`. Ahora gana el almacén,
// que es lo que hace que un `terraform output` guardado ayer sobreviva a un
// literal homónimo declarado como valor por defecto.
//
// PÉRDIDA ACEPTADA Y VISIBLE: el registro persiste HOY el mapa acumulado
// entero, no solo lo que el step produjo, así que un literal declarado entra en
// el almacén en la primera corrida y vuelve como `OriginState` en la segunda.
// Consecuencia: editar ese literal en el pipelinecode deja de surtir efecto —y
// el step ni siquiera se re-ejecuta, porque la huella de variables tampoco
// cambia—. Lo corrige la spec 14, al distinguir lo que un paso CONSUME de lo
// que PRODUCE (ver `fingerprint/SPEC-VARIABLES-v1.md` §2). Este test fija el
// borde: cuando la 14 llegue, se pone en rojo y la decisión se hace visible.
func TestRunCommand_ElAlmacenPisaAlLiteralDeclarado(t *testing.T) {
	h := newHarness(t)
	correHastaEstable(t, h)
	require.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])

	h.commitPipelineFile("variables/sand/supply.yaml", "- name: registry_prefix\n  value: vexsand2\n")

	h.resetLog()
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	assert.Empty(t, h.ranSteps(),
		"la huella de variables no cambia porque el valor efectivo no cambia")
	assert.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])
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
	stored := h.storedVars("sand", "02-supply")
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
	assert.Empty(t, h.storedVars("sand", "02-supply"))
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
	require.Len(t, control.cacheEntries(), 2, "control: dos pasos con comandos, dos entradas")

	h := newHarness(t, withPipelineFile("steps/02-supply/commands.yaml", ""))

	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, []string{"01-test"}, h.ranSteps(),
		"02-supply no ejecutó ningún comando")
	assert.Empty(t, h.persistedStepState("supply"),
		"un step saltado por falta de comandos no deja estado de re-ejecución")
	assert.Len(t, h.cacheEntries(), 1,
		"ni entrada de caché: sólo la del paso que sí corrió")
	assert.Empty(t, h.storedVars("sand", "02-supply"),
		"tampoco el almacén: registry_prefix estaba declarado, pero nada lo consumió")

	// Segunda corrida: vuelve a saltarse por la MISMA razón. Si hubiera
	// persistido estado, se saltaría por caché y las dos serían indistinguibles.
	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.NotContains(t, h.ranSteps(), "02-supply")
	assert.Empty(t, h.persistedStepState("supply"))
	assert.Len(t, h.cacheEntries(), 2,
		"las dos entradas son de 01-test —se re-ejecutó con otro material, ver "+
			"TestRunCommand_ReejecucionSinCambios—; 02-supply sigue sin dejar ninguna")
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
	assert.Len(t, h.cacheEntries(), 1,
		"una sola entrada, la del paso que terminó: no hay entrada de «se intentó»")

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
	assert.Len(t, h.cacheEntries(), 2, "una entrada por paso que terminó")
}

// ── El vocabulario de steps, abierto (specs 05 y 10 §5.3bis) ────────────────

// EL DIFF DE P1, y conviene mirarlo en la revisión: este test afirmaba lo
// CONTRARIO hasta la spec 10.
//
// Historia en tres actos. Hasta la spec 05, `05-notify` —estructuralmente
// impecable: prefijo de dos dígitos, orden único, comandos declarados— terminaba
// con exit code 0 sin haber corrido un solo comando suyo: `PolicyBuilder` le daba
// cero reglas y una policy sin reglas concluía «all rules passed». La spec 05
// convirtió aquel silencio en un fallo con nombre, como MEDIDA DE TRANSICIÓN y a
// sabiendas de que era corta.
//
// La spec 10 la retira, y no añadiendo nada: al borrar el `switch` que elegía
// comprobaciones por nombre de paso, la clave pasa a componerse del MATERIAL. Un
// paso desconocido no tiene entrada para su clave, luego se ejecuta; al terminar
// bien, escribe. El vocabulario de pasos se abre POR ELIMINACIÓN —es aquí, y no
// en la spec 15, que sólo añade la granularidad declarada—, y la medida de
// transición vivió exactamente entre la 05 y la 10.
func TestRunCommand_StepDesconocidoSeEjecutaYLaCorridaSiguienteLoSalta(t *testing.T) {
	const notifyCmd = "steps/05-notify/commands.yaml"
	const notifyBody = `
- name: avisar
  cmd: echo "05-notify AVISANDO" | tee -a "$VEX_TEST_LOG"
`

	h := newHarness(t, withPipelineFile(notifyCmd, notifyBody))

	result := h.run(withStep("notify"))

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.NotContains(t, result.stderr, "no tiene comprobaciones definidas",
		"el motor ya no necesita tener cableado el paso para saber si re-ejecutarlo")
	assert.Equal(t, []string{"01-test", "02-supply", "05-notify"}, h.ranSteps(),
		"un paso con un nombre que el motor no conoce se ejecuta como cualquier otro")
	assert.Len(t, h.cacheEntries(), 3, "y deja su entrada, como cualquier otro")

	// La segunda mitad, que es la que demuestra que no se ejecuta «siempre» sino
	// «cuando su contenido no consta»: la corrida siguiente lo salta.
	//
	// `05-notify` se salta ya en la segunda corrida porque no produce `outputs`;
	// `01-test` y `02-supply` sí, y por eso todavía necesitan una corrida más
	// (ver TestRunCommand_ReejecucionSinCambios).
	h.resetLog()
	segunda := h.run(withStep("notify"))
	require.Equal(t, cli.ExitSucceeded, segunda.exitCode, segunda.stderr)
	assert.NotContains(t, h.ranSteps(), "05-notify", "su contenido ya consta: se salta")
}

func TestRunCommand_StepDesconocidoPosteriorAlPedidoTampocoEstorba(t *testing.T) {
	// Este caso medía el LÍMITE del diagnóstico de la spec 05: era perezoso —vivía
	// en la cadena 2— así que un `05-notify` detrás de `02-supply` no se construía
	// y por tanto no se diagnosticaba, y `vex supply` devolvía 0 sobre un
	// pipelinecode que la 05 consideraba roto.
	//
	// Desde la spec 10 no hay nada que diagnosticar: el pipelinecode ya no está
	// roto. Se conserva el caso porque lo que sigue midiendo es real y no cambió:
	// la cadena de pipeline ejecuta `steps[:pedido+1]`, así que pedir `supply` no
	// toca `05-notify`.
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
	assert.Len(t, h.cacheEntries(), 2,
		"sin comandos no hay material que resumir: el paso no deja entrada")
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
	// limpieza de la spec 06 corre. Un `Ctrl-C` deja de ser muerte dura, que es
	// lo que estrecha la ventana descrita en la spec 09 §1 (a).
	assert.Empty(t, h.persistedStepState("test"),
		"un step cancelado no puede dejar escrito «sin cambios»")
	assert.Empty(t, h.cacheEntries(),
		"y no hay entrada de «se intentó»: una entrada existe si y sólo si un paso terminó bien")
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
