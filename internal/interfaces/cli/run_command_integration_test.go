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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/domain/syncconfig"
	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// correHastaEstable ejecuta hasta que ningún paso corre. Hoy hacen falta TRES
// ejecuciones, y por qué lo explica TestRunCommand_ReejecucionSinCambios.
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

	// Y que cada step persistió LO QUE ÉL PRODUJO, no todo lo que vio (spec 14
	// §6). Hasta esta spec los dos registros llevaban el mapa acumulado entero, y
	// de ahí salían los dos defectos que la spec cierra.
	stored := h.storedVars("sand", "02-supply")
	assert.Equal(t, "vexsand-demo-app", stored["acr_name"])
	assert.NotContains(t, stored, "artifact_name",
		"lo produjo 01-test: está en SU registro, y en el mapa acumulado durante la corrida")
	assert.NotContains(t, stored, "registry_prefix",
		"es un literal del pipelinecode: si se persistiera, editarlo dejaría de surtir efecto")

	assert.Equal(t, "demo-app", h.storedVars("sand", "01-test")["artifact_name"])

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
	// Motivo: cada step guarda en el almacén las variables que él mismo produjo
	// (`artifact_name`, `acr_name`). La ejecución siguiente las carga ANTES de
	// decidir, así que la huella de variables que se compara ya no es la que se
	// guardó, y el step se re-ejecuta exactamente una vez más. A partir de la
	// tercera el punto es fijo.
	//
	// La spec 14 acorta la cadena pero NO la corta: el registro dejó de guardar el
	// mapa acumulado entero —así que un literal declarado ya no vuelve del
	// almacén, ver `TestRunCommand_EditarUnLiteralVuelveASurtirEfecto`— pero lo
	// que un step PRODUJO sigue guardándose, que es su razón de ser, y sigue
	// entrando en la huella de la corrida siguiente porque el material de la
	// huella es el mapa acumulado RESUELTO.
	//
	// Lo cierra la spec 27, cuando ese material pase de «acumulado resuelto» a
	// «declarado» (§5.2): entonces las salidas de una corrida dejan de ser
	// entradas de la siguiente. Las dos specs son necesarias —la 14 hace que
	// «declarado» cubra también las variables que hoy aparecen de la nada, sin lo
	// cual el material quedaría corto EN SILENCIO— y este test es el testigo:
	// cuando la 27 llegue, la segunda ejecución se pondrá vacía.
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
	// Se aceptó porque la alternativa era peor: la selección por paso estaba
	// cableada en el motor POR NOMBRE, que es justo lo que P1 deroga.
	//
	// LA SPEC 15 DEVUELVE LA GRANULARIDAD, y este caso deja de medir una
	// regresión para medir una ELECCIÓN: los dos steps del fixture declaran
	// `- state_changed`, que incluye el proyecto, así que los dos se re-ejecutan
	// porque lo pidieron. Que un step pueda no pedirlo lo mide
	// `TestRunCommand_LoQueUnStepVigilaLoDeclaraElStep`.
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
// lee desde otro, y el ámbito de PROYECTO es otra clave.
//
// El aislamiento es el mismo que la spec 10 compró metiendo el ambiente en el
// hash. Lo que cambia es de dónde sale: ahora viaja en la clave de posición, así
// que no puede caerse de ningún hash por descuido.
//
// LA SEGUNDA MITAD CAMBIA CON LA SPEC 13 y conviene mirar el diff: hasta aquí
// `02-supply` escribía DOS registros —uno de proyecto, vacío, y otro del
// ambiente— porque el motor no sabía cuál era su ámbito. Ahora el step lo
// declara (`scope: environment` en su config.yaml) y escribe UNO.
func TestRunCommand_ElEstadoSeAislaPorAmbito(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run(withEnvironment("sand")).exitCode)
	require.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])

	assert.Empty(t, h.storedVars("prod", "02-supply"),
		"lo del ámbito de un ambiente no se lee desde otro")
	assert.Empty(t, h.projectVars("02-supply"),
		"y el ámbito de proyecto no se toca: el step declara el del ambiente")

	registros := strings.Join(h.persistedStepState("02-supply"), "\n")
	assert.Contains(t, registros, filepath.Join("environment", "sand", "02-supply"))
	assert.NotContains(t, registros, filepath.Join("project", "02-supply"),
		"un step, un ámbito, un registro: la bifurcación desapareció con un `if`")
}

// ── El step declara su ámbito (spec 13) ─────────────────────────────────────

// EL CASO QUE DA NOMBRE A LA SPEC: `scope` decide DÓNDE vive el registro, y sólo
// eso. El mismo pipelinecode, el mismo step, la misma huella; una línea de
// `config.yaml` de diferencia y el registro cambia de sitio.
func TestRunCommand_ElAmbitoDeclaradoDecideDondeViveElRegistro(t *testing.T) {
	t.Run("scope: project", func(t *testing.T) {
		h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml", configConScope("project")))

		require.Equal(t, cli.ExitSucceeded, h.run(withEnvironment("sand")).exitCode)

		assert.Equal(t, "vexsand-demo-app", h.projectVars("02-supply")["acr_name"])
		assert.Empty(t, h.storedVars("sand", "02-supply"),
			"nada bajo el ambiente: el step declaró que su trabajo es del proyecto")

		registros := strings.Join(h.persistedStepState("02-supply"), "\n")
		assert.Contains(t, registros, filepath.Join("project", "02-supply"))
		assert.NotContains(t, registros, filepath.Join("environment", "sand", "02-supply"))
	})

	t.Run("scope: environment", func(t *testing.T) {
		h := newHarness(t)

		require.Equal(t, cli.ExitSucceeded, h.run(withEnvironment("sand")).exitCode)

		assert.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])
		assert.Empty(t, h.projectVars("02-supply"))
	})
}

// LA RUPTURA DECLARADA de §5.7, hecha ejecutable. Un `workdir` cuyo primer
// segmento es `shared` marcaba las variables del comando como compartidas y las
// mandaba al ámbito de proyecto. Ya no: el ámbito lo declara el step, y este
// declara el del ambiente.
//
// Ningún template real estaba en este caso —los tres usan `./terraform/shared`,
// cuyo primer segmento es `.`— así que el cambio observable en producción es
// CERO. Lo que se rompe es una regla que nunca llegó a aplicarse.
func TestRunCommand_UnWorkdirLlamadoSharedYaNoComparte(t *testing.T) {
	h := newHarness(t,
		withPipelineFile("steps/02-supply/shared/terraform/.keep", ""),
		withPipelineFile("steps/02-supply/commands.yaml", `
- name: provision
  cmd: echo '02-supply acr_name = "desde-shared"' | tee -a "$VEX_TEST_LOG"
  workdir: shared/terraform
  outputs:
    - name: acr_name
      probe: acr_name = "([^"]+)"
`))

	result := h.run(withEnvironment("sand"))

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, "desde-shared", h.storedVars("sand", "02-supply")["acr_name"],
		"HOY iría al ámbito de proyecto: era el único workdir que activaba el mecanismo")
	assert.Empty(t, h.projectVars("02-supply"),
		"y `shared` deja de ser un nombre de directorio con significado para el motor")
}

// LECTURA CRUZADA (§5.4): se leen los dos ámbitos, se escribe en uno.
//
// Es el caso que motiva la spec entera. `01-test` declara ámbito de proyecto y
// publica `artifact_name`; `02-supply` es de ambiente y lo consume. Desde `prod`
// se ve lo que `01-test` dejó desplegando a `sand` —el ACR pertenece al
// proyecto—, mientras que lo que `02-supply` produjo en `sand` no cruza.
func TestRunCommand_UnStepDeAmbienteVeLoQueProdujoUnStepDeProyecto(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/01-test/config.yaml", configConScope("project")))

	require.Equal(t, cli.ExitSucceeded, h.run(withEnvironment("sand")).exitCode)
	require.Equal(t, "demo-app", h.projectVars("01-test")["artifact_name"],
		"01-test escribe en el ámbito de proyecto, sin ambiente en su clave")
	require.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])

	// Y ahora el mismo step de proyecto deja de producirlo: la ÚNICA fuente de
	// `artifact_name` pasa a ser su registro de ámbito de proyecto, escrito
	// desplegando a `sand`.
	h.commitPipelineFile("steps/01-test/commands.yaml",
		"- name: build\n  cmd: echo \"01-test BUILD\" | tee -a \"$VEX_TEST_LOG\"\n")

	h.resetLog()
	result := h.run(withEnvironment("prod"))

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, `02-supply acr_name = "vexprod-demo-app"`, h.logLines()[1],
		"prod ve el ámbito de proyecto aunque lo escribiera sand")

	// La otra mitad: lo del ambiente NO cruza. `acr_name` lo produjo 02-supply en
	// `sand` y su registro vive bajo `environment/sand`.
	assert.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])
	assert.Equal(t, "vexprod-demo-app", h.storedVars("prod", "02-supply")["acr_name"])
}

// §5.3, y es la mitad fácil de perder: SIN `config.yaml` el step se ejecuta
// SIEMPRE y `state/` no gana un solo archivo por él.
//
// Las dos cosas son la misma decisión. Ejecutar siempre es el default seguro
// —ejecutar de más nunca produce un despliegue que no ocurrió—, y no escribir es
// lo que impide inventarle un ámbito: asignarle `environment` por defecto sería
// la deducción implícita que esta spec retira, sólo que en el otro archivo.
func TestRunCommand_UnStepSinConfigSeEjecutaSiempreYNoPersiste(t *testing.T) {
	h := newHarness(t, withoutPipelineFile("steps/02-supply/config.yaml"))

	// Control: 01-test sí declara, así que la ausencia de abajo se ve.
	for range 3 {
		h.resetLog()
		result := h.run()
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		assert.Contains(t, h.ranSteps(), "02-supply",
			"sin ámbito no hay dónde constar que corrió, así que vuelve a correr")
	}

	assert.Empty(t, h.persistedStepState("02-supply"),
		"ni un registro: el motor no le inventa un ámbito")
	assert.Empty(t, h.storedVars("sand", "02-supply"))
	assert.Empty(t, h.projectVars("02-supply"))
	assert.NotEmpty(t, h.persistedStepState("01-test"),
		"control: el step que SÍ declara deja el suyo")

	// Sus variables siguen viajando en el mapa acumulado de la corrida: lo que no
	// ocurre es que crucen de una ejecución a la siguiente. `${var.artifact_name}`
	// lo produjo 01-test en ESTA corrida —o lo cargó de su registro, que sí
	// existe— y 02-supply lo interpola igual que siempre.
	assert.Contains(t, h.logLines(), `02-supply acr_name = "vexsand-demo-app"`)
}

// Un `scope` fuera del vocabulario cerrado aborta ANTES del primer step, con un
// mensaje que nombra el directorio. Es la misma disciplina que la spec 04: un
// pipelinecode roto descubierto a mitad del despliegue llega tarde, porque los
// steps anteriores ya tuvieron efectos reales.
func TestRunCommand_UnAmbitoInvalidoAbortaAntesDelPrimerStep(t *testing.T) {
	casos := []struct {
		nombre    string
		contenido string
		enElError string
	}{
		{
			nombre:    "un ámbito inventado",
			contenido: "scope: shared\n",
			enElError: "shared",
		},
		{
			nombre:    "config.yaml presente sin scope",
			contenido: "# sin nada declarado\n",
			enElError: "no declara 'scope'",
		},
		{
			nombre:    "config.yaml vacío",
			contenido: "",
			enElError: "no declara 'scope'",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml", caso.contenido))

			result := h.run()

			assert.Equal(t, cli.ExitFailed, result.exitCode)
			assert.Contains(t, result.stderr, "estructura del pipelinecode inválida")
			assert.Contains(t, result.stderr, "steps/02-supply/config.yaml",
				"el error nombra al culpable")
			assert.Contains(t, result.stderr, caso.enElError)
			assert.Empty(t, h.ranSteps(),
				"la validación corre antes del primer step: ningún despliegue queda a medias")
		})
	}
}

// ── Las reglas de re-ejecución declaradas (spec 15) ─────────────────────────

// EL PAR QUE DA NOMBRE A LA GRANULARIDAD DECLARADA, y son los tres tests que la
// spec 10 §9.8 dejó anunciados como «los que volverán a cambiar cuando la 15
// devuelva la granularidad».
//
// El caso concreto que lo hace cotidiano: un step que crea un registro de
// contenedores NO depende del código de la aplicación. Con «todo importa
// siempre» se re-ejecutaba en cada commit y en cada ambiente, y eso vaciaba de
// sentido haberlo separado por ámbito.
func TestRunCommand_LoQueUnStepVigilaLoDeclaraElStep(t *testing.T) {
	t.Run("state_changed: [pipeline] no ve el código del proyecto", func(t *testing.T) {
		h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml",
			"scope: environment\nrules:\n  - state_changed: [pipeline]\n"))
		correHastaEstable(t, h)

		h.writeProjectFile("src/app.txt", "v2\n")

		h.resetLog()
		result := h.run()
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

		assert.Equal(t, []string{"01-test"}, h.ranSteps(),
			"01-test sí lo vigila; 02-supply declaró que su trabajo no depende del código")
	})

	t.Run("con la forma corta sí lo ve", func(t *testing.T) {
		// El control, y es el fixture tal cual: mismo pipelinecode, mismo step,
		// misma huella de contenido; una línea de `config.yaml` de diferencia.
		h := newHarness(t)
		correHastaEstable(t, h)

		h.writeProjectFile("src/app.txt", "v2\n")

		h.resetLog()
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	})

	// La comprobación del default de §5.2, y el sitio donde un cambio de criterio
	// se vería: `- state_changed` y `- state_changed: [pipeline, project]` tienen
	// que producir decisiones IDÉNTICAS.
	//
	// La dirección importa: si la forma corta significara «sólo mi declaración»,
	// un `01-test` escrito así dejaría de re-ejecutarse ante un cambio de código.
	t.Run("la forma larga completa decide igual que la corta", func(t *testing.T) {
		h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml",
			"scope: environment\nrules:\n  - state_changed: [pipeline, project]\n"))
		correHastaEstable(t, h)

		h.writeProjectFile("src/app.txt", "v2\n")

		h.resetLog()
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	})
}

// EL CAMBIO DE COMPORTAMIENTO DE §5.7, fijado con test: **un step sin `max_age`
// no caduca**.
//
// Hasta aquí un `deploy` que llevaba 31 días sin tocarse se re-ejecutaba solo,
// por un TTL global que vivía en el motor. A partir de aquí eso ocurre si el
// pipelinecode lo pide, porque quien sabe cada cuánto conviene revisar un
// despliegue es quien lo escribió.
func TestRunCommand_SinMaxAgeUnRegistroNoCaduca(t *testing.T) {
	h := newHarness(t)
	correHastaEstable(t, h)

	h.envejecerRegistros(400 * 24 * time.Hour)

	h.resetLog()
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	assert.Empty(t, h.ranSteps(),
		"la misma huella y 400 días: HOY se re-ejecutaba por el TTL global de 30 días")
}

// La otra mitad: con `max_age` declarado, el registro sí caduca — y la ventana
// la elige el pipelinecode, no el motor.
func TestRunCommand_MaxAgeDeclaradoCaducaElRegistro(t *testing.T) {
	const conVentana = "scope: environment\nrules:\n  - state_changed\n  - max_age: %s\n"

	t.Run("fuera de la ventana se ejecuta, con la huella intacta", func(t *testing.T) {
		h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml",
			fmt.Sprintf(conVentana, "1h")))
		correHastaEstable(t, h)

		h.envejecerRegistros(2 * time.Hour)

		h.resetLog()
		result := h.run()
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

		assert.Equal(t, []string{"02-supply"}, h.ranSteps(),
			"sólo el que declara la ventana: 01-test no caduca porque no lo pidió")
	})

	t.Run("dentro de la ventana revive", func(t *testing.T) {
		h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml",
			fmt.Sprintf(conVentana, "24h")))
		correHastaEstable(t, h)

		h.envejecerRegistros(2 * time.Hour)

		h.resetLog()
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		assert.Empty(t, h.ranSteps())
	})
}

// EL OR, de punta a punta: cada regla es una razón INDEPENDIENTE para desconfiar
// de lo guardado, así que basta con que una se cumpla.
func TestRunCommand_LasReglasSeCombinanConOr(t *testing.T) {
	const dosReglas = "scope: environment\nrules:\n  - state_changed\n  - max_age: 24h\n"

	t.Run("la huella cambia dentro de la ventana", func(t *testing.T) {
		h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml", dosReglas))
		correHastaEstable(t, h)

		h.writeProjectFile("src/app.txt", "v2\n")

		h.resetLog()
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
		assert.Contains(t, h.ranSteps(), "02-supply")
	})

	t.Run("la huella no cambia y la ventana pasa", func(t *testing.T) {
		h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml", dosReglas))
		correHastaEstable(t, h)

		h.envejecerRegistros(48 * time.Hour)

		h.resetLog()
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
		assert.Equal(t, []string{"02-supply"}, h.ranSteps())
	})
}

// §5.5, y es la invariante de la spec 05 trasladada a una forma nueva: **sin
// reglas se ejecuta, nunca se revive**. Y, como el step sin `config.yaml` y como
// el step sin comandos, tampoco escribe registro.
//
// El OR de un conjunto vacío es falso, así que la lectura literal diría
// «revivir». «No hay nada que comprobar» no es «esta configuración está al día»:
// concluir a partir de un conjunto vacío de evidencia es la única forma en que
// este motor puede omitir un despliegue en silencio.
func TestRunCommand_UnStepSinReglasSeEjecutaSiempreYNoPersiste(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml", "scope: environment\n"))

	for range 3 {
		h.resetLog()
		result := h.run()
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		assert.Contains(t, h.ranSteps(), "02-supply",
			"declara dónde recordarse y ninguna razón para confiar en lo recordado")
	}

	assert.Empty(t, h.persistedStepState("02-supply"),
		"ni un registro: no hay afirmación que guardar")
	assert.Empty(t, h.storedVars("sand", "02-supply"))
	assert.NotEmpty(t, h.persistedStepState("01-test"),
		"control: el step que SÍ declara reglas deja el suyo")

	// Y lo que produce sigue viajando por el mapa acumulado durante la corrida:
	// lo que no ocurre es que cruce de una ejecución a la siguiente.
	assert.Contains(t, h.logLines(), `02-supply acr_name = "vexsand-demo-app"`)
}

// LA CONFIGURACIÓN VÁLIDA Y PELIGROSA de §5.4: sólo expiración. **No es un error
// y no se prohíbe** —nada es implícito, todo se declara, y no existe una
// comprobación que el motor imponga por fuera de lo que el `config.yaml` dice—.
//
// Lo que este caso mide es la CONSECUENCIA, que es lo observable desde fuera: el
// step revive un resultado obsoleto durante toda su ventana de vigencia aunque su
// contenido haya cambiado de forma evidente. Que además se AVISE lo fija
// `TestStepRunnerHandler_ExpirarSinInvalidarAvisaYNoAborta`: el aviso viaja por
// el emisor de log, que el harness silencia con `--quiet` para no escribir en el
// os.Stdout del proceso de test.
func TestRunCommand_SoloConMaxAgeUnStepRevivePeseAlCambio(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml",
		"scope: environment\nrules:\n  - max_age: 24h\n"))

	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	require.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

	h.commitPipelineFile("steps/02-supply/commands.yaml", `
- name: provision
  cmd: echo '02-supply acr_name = "otro-del-todo"' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: acr_name
      probe: acr_name = "([^"]+)"
`)

	h.resetLog()
	segunda := h.run()

	require.Equal(t, cli.ExitSucceeded, segunda.exitCode, segunda.stderr)
	assert.NotContains(t, h.ranSteps(), "02-supply",
		"sus comandos son OTROS y ninguna regla suya mira el contenido: de esto avisa §5.4")
}

// La gramática de `rules` aborta ANTES del primer step, y sin una regla nueva en
// el validador: el repositorio valida al TRADUCIR y el validador de la spec 04 lo
// llama para cada step antes del primero.
//
// Es la misma disciplina de siempre: un pipelinecode roto descubierto a mitad del
// despliegue llega tarde, porque los steps anteriores ya tuvieron efectos reales.
func TestRunCommand_UnaReglaInvalidaAbortaAntesDelPrimerStep(t *testing.T) {
	casos := []struct {
		nombre    string
		contenido string
		enElError string
	}{
		{
			nombre:    "una regla que el motor no conoce",
			contenido: "scope: environment\nrules:\n  - content_changed\n",
			enElError: "content_changed",
		},
		{
			nombre:    "state_changed sin pipeline",
			contenido: "scope: environment\nrules:\n  - state_changed: [project]\n",
			enElError: "no puede omitir 'pipeline'",
		},
		{
			nombre:    "una duración en días",
			contenido: "scope: environment\nrules:\n  - max_age: 30d\n",
			enElError: "720h",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			h := newHarness(t, withPipelineFile("steps/02-supply/config.yaml", caso.contenido))

			result := h.run()

			assert.Equal(t, cli.ExitFailed, result.exitCode)
			assert.Contains(t, result.stderr, "estructura del pipelinecode inválida")
			assert.Contains(t, result.stderr, "steps/02-supply/config.yaml",
				"el error nombra al culpable")
			assert.Contains(t, result.stderr, caso.enElError)
			assert.Empty(t, h.ranSteps(),
				"la validación corre antes del primer step: ningún despliegue queda a medias")
		})
	}
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
	assert.Equal(t, "demo-app", h.storedVars("sand", "01-test")["artifact_name"],
		"el valor vive en el registro de quien lo produjo (spec 14 §6)")
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

	// Y el registro NUEVO ya no lo lleva: el step dejó de producirlo, así que
	// dejó de ser un hecho suyo (spec 14 §6). El de la primera corrida sigue ahí
	// —el almacén es append-only— pero el vigente es éste.
	assert.NotContains(t, h.storedVars("sand", "02-supply"), "project_name")
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

// EDITAR UN LITERAL VUELVE A SURTIR EFECTO. Es el defecto que la spec 14 cierra,
// y este test es su testigo: se llamaba `TestRunCommand_ElAlmacenPisaAlLiteral‐
// Declarado` y afirmaba lo contrario.
//
// La cadena del defecto tenía dos eslabones y esta spec corta el primero:
//
//	el registro guardaba el mapa acumulado ENTERO          ← la spec 14 lo corta
//	  ⇒ el literal entraba en el almacén en la 1ª corrida
//	  ⇒ volvía como `OriginState` en la 2ª, por encima de `OriginDeclared`
//	  ⇒ el valor efectivo no cambiaba al editarlo
//	  ⇒ la huella de variables tampoco, así que el step ni se re-ejecutaba
//
// La spec 12 no lo podía cerrar sola —invertir la precedencia es lo que lo hizo
// visible— y la 27 tampoco: hashear la declaración en vez del valor resuelto hace
// que editar un literal vuelva a RE-EJECUTAR el step, no que el literal vuelva a
// GANAR. Hacen falta las dos, y el reparto es: la 27 arregla la identidad, ésta
// arregla el valor.
//
// Los ~40 literales de `variables/<env>/deploy.yaml` de los templates estaban en
// esta situación.
func TestRunCommand_EditarUnLiteralVuelveASurtirEfecto(t *testing.T) {
	h := newHarness(t)
	correHastaEstable(t, h)
	require.Equal(t, "vexsand-demo-app", h.storedVars("sand", "02-supply")["acr_name"])

	h.commitPipelineFile("variables/sand/supply.yaml", "- name: registry_prefix\n  value: vexsand2\n")

	h.resetLog()
	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	assert.Contains(t, h.ranSteps(), "02-supply",
		"el literal ya no vuelve del almacén, así que el valor efectivo cambia y con él la huella")
	assert.Equal(t, `02-supply acr_name = "vexsand2-demo-app"`, h.logLines()[len(h.logLines())-1])
	assert.Equal(t, "vexsand2-demo-app", h.storedVars("sand", "02-supply")["acr_name"])
}

// La otra mitad de la spec 12, que NO cambia: lo que una ejecución anterior
// produjo sigue ganando a un literal homónimo declarado como valor por defecto.
// Es lo que hace que un `terraform output` guardado ayer sobreviva.
//
// Lo que la spec 14 le quita al almacén no es autoridad: es la copia de los
// literales que acababan dentro de él sin que nadie los hubiera producido.
func TestRunCommand_ElAlmacenSigueGanandoALoDeclarado(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/02-supply/commands.yaml", `
- name: provision
  cmd: echo '02-supply acr_name = "de-la-nube"' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: acr_name
      probe: acr_name = "([^"]+)"
`))

	// Corrida 1: el step produce `acr_name` y lo guarda en su registro.
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	require.Equal(t, "de-la-nube", h.storedVars("sand", "02-supply")["acr_name"])

	// El step deja de producirlo y alguien declara un literal con ese nombre. El
	// almacén es ahora la única fuente que compite, y gana.
	h.commitPipelineFile("variables/sand/supply.yaml",
		"- name: registry_prefix\n  value: vexsand\n- name: acr_name\n  value: literal-por-defecto\n")
	h.commitPipelineFile("steps/02-supply/commands.yaml",
		"- name: usa\n  cmd: echo '02-supply acr_name=${var.acr_name}' | tee -a \"$VEX_TEST_LOG\"\n")

	h.resetLog()
	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Contains(t, h.logLines(), "02-supply acr_name=de-la-nube",
		"un literal es un valor por DEFECTO: no pisa un hecho que una corrida anterior registró")
}

// ── El consumidor declara el origen (spec 14) ───────────────────────────────
//
// Hasta esta spec una variable viajaba entre steps así: `01-test` la extraía del
// stdout y la inyectaba en un mapa global plano, y tres steps más tarde alguien
// la interpolaba POR NOMBRE. El consumidor no declaraba de dónde venía, sólo la
// nombraba, y el acoplamiento se resolvía por orden de ejecución — o sea, no era
// conocible antes de ejecutar, que es justo lo que la identidad necesita saber.

// manifiestoV2 sustituye al `vexpipeline.yaml` del fixture, así que tiene que
// conservar su `clone_window`: sin ella el pipelinecode volvería a la ventana de
// 24 h y los casos que commitean un cambio y vuelven a ejecutar reutilizarían el
// clon sin verlo (spec 18 §5.4).
const manifiestoV2 = "schema_version: 2\n" + ventanaDelFixture

// La versión del formato existe para que añadir gramática no rompa a nadie en
// silencio. Sin `vexpipeline.yaml` el pipelinecode está en la versión 1, y en la
// 1 `resolve` no existe: el motor lo dice nombrando la versión que haría falta, y
// lo dice ANTES del primer step.
func TestRunCommand_SinManifiestoResolveSeRechaza(t *testing.T) {
	h := newHarness(t, withPipelineFile("variables/sand/supply.yaml", `
- name: registry_prefix
  value: vexsand
- name: artifact
  resolve: step-output
  from: "01-test"
  key: artifact_name
`))

	result := h.run()

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "schema_version: 2")
	assert.Contains(t, result.stderr, "vexpipeline.yaml")
	assert.Empty(t, h.ranSteps(), "falla en la carga: ningún step tuvo efectos reales")
}

// Con la versión declarada, la gramática funciona: el consumidor nombra el step
// productor y el output, y puede llamar a la variable como quiera. El productor
// deja de tener que saber quién lo lee — es el DIP aplicado al pipelinecode.
func TestRunCommand_ConVersion2UnStepOutputSeResuelve(t *testing.T) {
	h := newHarness(t,
		withPipelineFile("vexpipeline.yaml", manifiestoV2),
		withPipelineFile("variables/sand/supply.yaml", `
- name: registry_prefix
  value: vexsand
- name: artefacto
  resolve: step-output
  from: "01-test"
  key: artifact_name
`),
		withPipelineFile("steps/02-supply/commands.yaml", `
- name: provision
  cmd: echo '02-supply acr_name = "${var.registry_prefix}-${var.artefacto}"' | tee -a "$VEX_TEST_LOG"
  outputs:
    - name: acr_name
      probe: acr_name = "([^"]+)"
`))

	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, `02-supply acr_name = "vexsand-demo-app"`, h.logLines()[1])
}

// Las dos validaciones de grafo de §5.4, vistas desde fuera. Las dos son
// posibles sólo porque `from` hace explícito el grafo de dependencias, y las dos
// mueven un fallo de EJECUCIÓN —a mitad del despliegue, con `test` ya ejecutado—
// a un fallo de CARGA.
func TestRunCommand_UnGrafoDeVariablesInvalidoAbortaAntesDelPrimerStep(t *testing.T) {
	casos := []struct {
		nombre    string
		archivo   string
		contenido string
		enElError string
		nota      string
	}{
		{
			nombre:  "from apunta a un step posterior",
			archivo: "variables/sand/test.yaml",
			contenido: "- name: acr\n  resolve: step-output\n" +
				"  from: \"02-supply\"\n  key: acr_name\n",
			enElError: "no se ejecuta antes",
			nota:      "HOY falla en runtime con «variable no existe», después de que 01-test corrió",
		},
		{
			nombre:  "from apunta a un step que no existe",
			archivo: "variables/sand/supply.yaml",
			contenido: "- name: registry_prefix\n  value: vexsand\n" +
				"- name: acr\n  resolve: step-output\n  from: \"07-inventado\"\n  key: acr_name\n",
			enElError: "steps/07-inventado",
		},
		{
			nombre:  "key que ningún outputs declara",
			archivo: "variables/sand/supply.yaml",
			contenido: "- name: registry_prefix\n  value: vexsand\n" +
				"- name: acr\n  resolve: step-output\n  from: \"01-test\"\n  key: artefacto\n",
			enElError: "key: artefacto",
			nota:      "HOY lo resolvería otro step que casualmente declaró el mismo nombre",
		},
		{
			nombre:  "un resolve inventado",
			archivo: "variables/sand/supply.yaml",
			contenido: "- name: registry_prefix\n  value: vexsand\n" +
				"- name: acr\n  resolve: dynamic\n",
			enElError: "'resolve: dynamic'",
			nota:      "el vocabulario es cerrado: un genérico no discriminaría nada",
		},
		{
			nombre:  "un step-output sin key",
			archivo: "variables/sand/supply.yaml",
			contenido: "- name: registry_prefix\n  value: vexsand\n" +
				"- name: acr\n  resolve: step-output\n  from: \"01-test\"\n",
			enElError: "'key'",
			nota:      "los campos obligatorios de cada origen son un error, no un default",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			h := newHarness(t,
				withPipelineFile("vexpipeline.yaml", manifiestoV2),
				withPipelineFile(caso.archivo, caso.contenido))

			result := h.run()

			assert.Equal(t, cli.ExitFailed, result.exitCode, caso.nota)
			assert.Contains(t, result.stderr, caso.enElError, caso.nota)
			assert.Empty(t, h.ranSteps(),
				"tiene que fallar antes del primer step: %s", caso.nota)
		})
	}
}

// Una versión de esquema que este motor no entiende es un error, y no una
// interpretación a medias.
func TestRunCommand_UnaVersionDeFormatoDesconocidaSeRechaza(t *testing.T) {
	h := newHarness(t, withPipelineFile("vexpipeline.yaml", "schema_version: 99\n"))

	result := h.run()

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "schema_version: 99")
	assert.Empty(t, h.ranSteps())
}

// `resolve: state` lee del registro del propio step bajo OTRO ámbito. Es la
// mitad lectora de la asimetría de la spec 13 §5.4 —se leen los dos ámbitos, se
// escribe en uno— dicha en voz alta en vez de ocurrir sola por el orden de dos
// cargas.
func TestRunCommand_UnaVariableDeEstadoSeResuelvePorDeclaracion(t *testing.T) {
	h := newHarness(t,
		withPipelineFile("vexpipeline.yaml", manifiestoV2),
		withPipelineFile("steps/02-supply/config.yaml", configConScope("project")))

	// Corrida 1: 02-supply produce `acr_name` y lo registra en el ámbito de
	// proyecto, que es el que declara.
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	require.Equal(t, "vexsand-demo-app", h.projectVars("02-supply")["acr_name"])

	// Corrida 2: lo consume por declaración, con otro nombre.
	h.commitPipelineFile("variables/sand/supply.yaml", `
- name: registry_prefix
  value: vexsand
- name: registro_anterior
  resolve: state
  scope: project
  key: acr_name
`)
	h.commitPipelineFile("steps/02-supply/commands.yaml",
		"- name: usa\n  cmd: echo '02-supply anterior=${var.registro_anterior}' | tee -a \"$VEX_TEST_LOG\"\n")

	h.resetLog()
	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Contains(t, h.logLines(), "02-supply anterior=vexsand-demo-app")
}

// Y cuando la fuente NO produjo su valor, el error dice QUÉ FUENTE falló.
//
// Es la diferencia observable de §5.5: hasta esta spec el handler de variables
// trataba todo fallo de interpolación como «aún no resoluble», sin distinguir un
// error real de plantilla de una variable que llegaría más tarde. No podía
// distinguirlos porque nadie había declarado cuál es cuál, así que las dos
// terminaban en «variable no existe» — un mensaje que no dice a quién reclamarle.
func TestRunCommand_UnaFuenteQueNoProduceNombraLaFuente(t *testing.T) {
	h := newHarness(t,
		withPipelineFile("vexpipeline.yaml", manifiestoV2),
		withPipelineFile("steps/02-supply/config.yaml", configConScope("project")),
		withPipelineFile("variables/sand/supply.yaml", `
- name: registry_prefix
  value: vexsand
- name: registro_anterior
  resolve: state
  scope: project
  key: lb-arn
`))

	result := h.run()

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "lb-arn", "el error nombra la CLAVE que falta")
	assert.Contains(t, result.stderr, "project", "y el ámbito donde se buscó")
	assert.NotContains(t, result.stderr, "variable no existe")
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

	// Lo que el almacén ya NO guarda desde la spec 14 es el literal: un registro
	// dice qué produjo el step, y `instance_count` lo consumió (§6). Que
	// «declarada y vacía» siga siendo distinto de «no declarada» se observa donde
	// importa —en el material de la huella y en la interpolación de arriba—, no en
	// una copia dentro del almacén; esa copia era justamente la que hacía que
	// editar un literal dejara de surtir efecto.
	stored := h.storedVars("sand", "02-supply")
	assert.NotContains(t, stored, "")
	assert.NotContains(t, stored, "instance_count")
	assert.Equal(t, "vexsand-demo-app", stored["acr_name"])
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

// EL DIFF DE D9, y conviene mirarlo: este test se llamaba
// `TestRunCommand_AmbienteLlamadoShared` y afirmaba lo CONTRARIO.
//
// La spec 04 §5.4 reservó `shared` porque el ámbito del almacén compartido
// ocupaba la misma posición que el ambiente en la ruta, y un ambiente así
// llamado lo pisaba. La spec 11 cambió la clave —el ámbito de ambiente viaja
// SIEMPRE prefijado, `environment:<nombre>`— y la 13 §5.5 saca la conclusión:
// **no queda ninguna palabra reservada**. Un ambiente `shared` da
// `environment/shared` y uno `project` da `environment/project`; ninguno
// colisiona con el ámbito de proyecto, que vive en `project/`.
//
// Es el resultado que la spec 04 no podía dar: aquella arregló la colisión
// prohibiendo un nombre; ésta la elimina cambiando la clave, que es la
// corrección que no le cuesta nada al usuario.
func TestRunCommand_NingunaPalabraDelUsuarioEstaReservada(t *testing.T) {
	for _, ambiente := range []string{"shared", "project"} {
		t.Run(ambiente, func(t *testing.T) {
			h := newHarness(t,
				withPipelineFile("environments.yaml",
					"- name: Reservado\n  value: "+ambiente+"\n- name: Sandbox\n  value: sand\n"),
				withPipelineFile("variables/"+ambiente+"/supply.yaml",
					"- name: registry_prefix\n  value: vexraro\n"))

			result := h.run(withEnvironment(ambiente))

			require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
			assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

			assert.Equal(t, "vexraro-demo-app", h.storedVars(ambiente, "02-supply")["acr_name"],
				"su registro va a environment/%s y no pisa nada", ambiente)
			assert.Empty(t, h.projectVars("02-supply"),
				"el ámbito de proyecto vive en project/, sin prefijo que colisione")
		})
	}
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

	// Declara ámbito como cualquier otro step: lo que este caso mide es que el
	// motor no necesita conocer su NOMBRE, no que un step sin `config.yaml`
	// persista (spec 13 §5.3, y de eso habla
	// `TestRunCommand_UnStepSinConfigSeEjecutaSiempreYNoPersiste`).
	h := newHarness(t,
		withPipelineFile(notifyCmd, notifyBody),
		withPipelineFile("steps/05-notify/config.yaml", configConScope("environment")))

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

// LA RUPTURA DE CONTRATO de la spec 16 §5.7: un cliente de la v1 se rechaza, y
// el mensaje dice QUÉ cambió.
//
// Subir el número es el gatillo que impide que el «por ahora» se vuelva
// permanente: sin él, el CLI `vex` y las edge functions seguirían pasando
// `--mode` y los seis endpoints, el motor los ignoraría en silencio y el
// desajuste aparecería como un despliegue que no revive nunca. Y sin la segunda
// línea del mensaje, el diagnóstico del día en que se retome la CLI (spec 23)
// cuesta una tarde.
func TestRunCommand_SchemaVersion1SeRechazaYElMensajeDiceQueCambio(t *testing.T) {
	h := newHarness(t)

	result := h.run(withSchemaVersion(1))

	assert.Equal(t, cli.ExitInputError, result.exitCode)
	assert.Contains(t, result.stderr, "unsupported schema_version: 1")
	assert.Contains(t, result.stderr, "--mode")
	assert.Contains(t, result.stderr, "--state-config")
	assert.Contains(t, result.stderr, cli.StateConfigEnvVar)
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

// EL SILENCIO SE ACABA, y sustituir este caso por su contrario es parte del
// alcance de la spec 19 (§7), no un test roto que se borra.
//
// Aquí vivía `TestRunCommand_ObserverDeStatusNuncaRecibeStages`, una
// CARACTERIZACIÓN escrita en la spec 01 §9.2: con `--quiet=false` el
// `StdoutStatusObserver` se componía sobre el writer y nadie lo notificaba
// jamás, así que el writer se quedaba vacío. `NotifyStage` estaba cableado de
// punta a punta y `grep` no encontraba un solo llamador (BL-5/R-10). Aquel test
// se escribió para ponerse en rojo el día que eso cambiara, y este es el día.
//
// Se CABLEA y no se borra porque `--status-endpoint` sobrevive (spec 16) y es el
// canal que el portal usa: darle una fuente real es más barato que mantener dos
// vocabularios. La etapa la emite el dueño del hecho de apertura del step, en el
// mismo punto y derivada de él.
func TestRunCommand_ElObserverDeStatusRecibeUnaEtapaPorStep(t *testing.T) {
	h := newHarness(t)

	args := h.args()
	args.Quiet = false
	result := h.execute(args, strings.NewReader(string(h.marshal(h.request()))))

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	etapas := make([]string, 0, 2)
	for _, linea := range strings.Split(strings.TrimSpace(result.stdout), "\n") {
		etapas = append(etapas, strings.TrimPrefix(strings.TrimSpace(linea), "→ "))
	}
	assert.Equal(t, []string{"running_step:01-test", "running_step:02-supply"}, etapas,
		"una etapa por step, en orden, y con el identificador completo del step")
}

// ── El destino del estado es configuración (spec 16) ────────────────────────

// EL CASO QUE DA NOMBRE A LA SPEC, y es una ausencia: **sin configuración de
// destino el motor no arranca**.
//
// Hasta aquí `--mode` traía un default —`remote`, o sea «Supabase»— dentro de la
// pieza que tiene que ser portable: invocar el motor sin decir nada asumía una
// plataforma concreta, y el motor se comportaba distinto según quién lo invocó
// sin que quedara escrito en ningún lado. El default vive ahora en quien invoca.
func TestRunCommand_SinConfiguracionDeDestinoElMotorNoArranca(t *testing.T) {
	h := newHarness(t)

	args := h.args()
	args.StateConfigFile = ""

	_, err := h.build(args)

	require.Error(t, err)
	assert.Equal(t, cli.ExitInputError, cli.ExitCodeFor(err),
		"no es un fallo de la pipeline: es una invocación que no se puede atender")
	assert.Contains(t, err.Error(), "--state-config", "el mensaje nombra la flag")
	assert.Contains(t, err.Error(), cli.StateConfigEnvVar, "y la otra vía")
	assert.Contains(t, err.Error(), "schema_version 2",
		"y la versión, que es la pista que ahorra la tarde de diagnóstico")
	assert.Empty(t, h.ranSteps())
}

// Las dos vías del transporte, que son las de RequestInput menos stdin (§5.2).
// Se reutiliza el mecanismo existente en vez de inventar uno porque la Fly
// Machine ya pasa el input por entorno.
func TestRunCommand_ViasDeLaConfiguracionDeDestino(t *testing.T) {
	t.Run("--state-config", func(t *testing.T) {
		h := newHarness(t)
		result := h.run()
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	})

	t.Run("env var con YAML crudo", func(t *testing.T) {
		h := newHarness(t)
		t.Setenv(cli.StateConfigEnvVar, "type: local\nlocal:\n  path: "+h.destino+"\n")

		args := h.args()
		args.StateConfigFile = ""
		args.InputFile = h.writeRequest(h.request())
		result := h.execute(args, nil)

		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())
	})

	t.Run("env var con JSON crudo", func(t *testing.T) {
		// El contrato está escrito en YAML y quien lo genere desde código lo hará
		// en JSON. El decodificador acepta los dos porque JSON es YAML.
		h := newHarness(t)
		t.Setenv(cli.StateConfigEnvVar,
			fmt.Sprintf(`{"type":"local","local":{"path":%q}}`, h.destino))

		args := h.args()
		args.StateConfigFile = ""
		args.InputFile = h.writeRequest(h.request())
		result := h.execute(args, nil)

		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	})

	t.Run("env var con base64", func(t *testing.T) {
		h := newHarness(t)
		t.Setenv(cli.StateConfigEnvVar, base64.StdEncoding.EncodeToString(
			[]byte("type: local\nlocal:\n  path: "+h.destino+"\n")))

		args := h.args()
		args.StateConfigFile = ""
		args.InputFile = h.writeRequest(h.request())
		result := h.execute(args, nil)

		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	})

	t.Run("--state-config gana sobre la env var", func(t *testing.T) {
		h := newHarness(t)
		t.Setenv(cli.StateConfigEnvVar, "type: http\nhttp:\n  endpoint: https://x.test\n")

		result := h.run()
		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	})
}

// §5.5: `http` está en el vocabulario y NO está implementado, y el error lo dice
// con esas palabras. Es una decisión con fecha abierta, no un hueco — y por eso
// tiene que distinguirse de una configuración malformada, que es un error de
// quien la escribió.
func TestRunCommand_TypeHttpEstaCongeladoYSeDistingueDeUnErrorDeFormato(t *testing.T) {
	h := newHarness(t)

	t.Run("congelado", func(t *testing.T) {
		writeFile(t, h.stateConfig, "type: http\nhttp:\n  endpoint: https://ingesta.vex.test\n")

		_, err := h.build(h.args())

		require.Error(t, err)
		assert.Equal(t, cli.ExitInputError, cli.ExitCodeFor(err))
		assert.ErrorIs(t, err, syncconfig.ErrCongelado)
	})

	t.Run("malformada", func(t *testing.T) {
		writeFile(t, h.stateConfig, "type: supabase\n")

		_, err := h.build(h.args())

		require.Error(t, err)
		assert.Equal(t, cli.ExitInputError, cli.ExitCodeFor(err))
		assert.NotErrorIs(t, err, syncconfig.ErrCongelado,
			"un vocabulario equivocado no es «todavía no implementado»")
	})
}

// **`type: local` NO es un Null Object**, y este es el test que lo fija. No se
// puede garantizar que el volumen esté montado; si no lo está, escribir a ciegas
// dejaría al motor registrando en el filesystem efímero del contenedor sin
// ninguna señal. Y lo que se perdería no es velocidad: es el identificador del
// recurso que se acaba de crear en la nube.
func TestRunCommand_UnDestinoInservibleFallaRuidosamente(t *testing.T) {
	t.Run("la ruta no existe: el volumen no está montado", func(t *testing.T) {
		h := newHarness(t)
		inexistente := filepath.Join(t.TempDir(), "volumen-no-montado")
		writeFile(t, h.stateConfig, "type: local\nlocal:\n  path: "+inexistente+"\n")

		_, err := h.build(h.args())

		require.Error(t, err)
		assert.Equal(t, cli.ExitInputError, cli.ExitCodeFor(err))
		assert.Contains(t, err.Error(), inexistente, "el error nombra la ruta")
		assert.Contains(t, err.Error(), "volumen")
		assert.Empty(t, h.ranSteps(), "y falla antes del primer step")
	})

	t.Run("la ruta no es escribible", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("como root los permisos no impiden escribir, y el caso mide justo eso")
		}
		h := newHarness(t)
		soloLectura := filepath.Join(t.TempDir(), "solo-lectura")
		require.NoError(t, os.MkdirAll(soloLectura, 0o555))
		t.Cleanup(func() { _ = os.Chmod(soloLectura, 0o755) })
		writeFile(t, h.stateConfig, "type: local\nlocal:\n  path: "+soloLectura+"\n")

		_, err := h.build(h.args())

		require.Error(t, err)
		assert.Equal(t, cli.ExitInputError, cli.ExitCodeFor(err))
		assert.Contains(t, err.Error(), "no es escribible")
	})

	t.Run("la ruta es un archivo", func(t *testing.T) {
		h := newHarness(t)
		archivo := filepath.Join(t.TempDir(), "no-soy-un-directorio")
		writeFile(t, archivo, "")
		writeFile(t, h.stateConfig, "type: local\nlocal:\n  path: "+archivo+"\n")

		_, err := h.build(h.args())

		require.Error(t, err)
		assert.Equal(t, cli.ExitInputError, cli.ExitCodeFor(err))
	})
}

// EL ESTADO DEJA DE SER DE UNA MÁQUINA, que es a lo que la spec entera apunta.
//
// La mitad de esta comprobación ya existía —`LaClaveNoDependeDeLaMaquina`: el
// mismo árbol desde dos $HOME produce las MISMAS claves—, pero hasta aquí las dos
// máquinas no tenían dónde encontrarse. Con el destino como dato, la segunda
// máquina lee lo que escribió la primera y REVIVE, sin haber ejecutado nada.
//
// Es también donde muerde el defecto heredado de `.git` como archivo (spec 08
// §9.10): en un worktree, `.git` entra en la huella con una ruta absoluta dentro
// y este caso se pondría en rojo. El fixture usa repos normales, así que hoy pasa
// — y como está escrito, el día que se decida la v2 de la regla del árbol la
// decisión no se puede tomar por omisión.
func TestRunCommand_DosMaquinasQueComparteElDestinoSeRevivenEntreSi(t *testing.T) {
	primera := newHarness(t)
	correHastaEstable(t, primera)
	entradas := primera.cacheEntries()
	registros := primera.persistedStepState("02-supply")
	require.NotEmpty(t, entradas)

	// Otro $HOME, otra ruta absoluta para el árbol del proyecto, el mismo destino.
	segunda := primera.otraMaquina()
	segunda.compartiendoElDestinoCon(primera)

	segunda.resetLog()
	result := segunda.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Empty(t, segunda.ranSteps(),
		"la segunda máquina acierta el registro de la primera: el estado no es de una máquina")
	assert.Equal(t, entradas, segunda.cacheEntries(),
		"y revivir no escribe: ni una entrada nueva")
	assert.Equal(t, registros, segunda.persistedStepState("02-supply"))
}

// La otra mitad, y es la que hace observable que el estado CAMBIÓ DE SITIO: bajo
// el $HOME del proceso no queda nada. Si quedara, dos máquinas seguirían
// escribiendo cada una en su rincón y el caso de arriba pasaría por accidente.
func TestRunCommand_ElEstadoNoCuelgaDelHomeDelProceso(t *testing.T) {
	h := newHarness(t)
	correHastaEstable(t, h)

	require.NotEmpty(t, h.persistedStepState("02-supply"), "control: el estado se escribió")

	assert.NoDirExists(t, filepath.Join(h.root, cli.VexHomeDirName, "state"))
	assert.NoDirExists(t, filepath.Join(h.root, cli.VexHomeDirName, "cache"))
	assert.DirExists(t, filepath.Join(h.root, cli.VexHomeDirName, "projects"),
		"lo que sigue bajo el $HOME es el área de clones, que es trabajo y no verdad")
}

// §5.6, y es la corrección a I-6 hecha test de regresión: **`--status-endpoint`
// sobrevive**. La revisión proponía retirarlo con los seis del caché, dando por
// hecho que la pérdida era de rendimiento; el código dice otra cosa. Es la única
// señal que el portal tiene de que el contenedor terminó, así que retirarlo con
// `type: http` congelado dejaría toda ejecución remota en `running` hasta que un
// TTL la marcara `error`.
func TestRunCommand_ElStatusTerminalSigueLlegandoASuEndpoint(t *testing.T) {
	var mu sync.Mutex
	recibidos := make([]map[string]any, 0, 2)

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		mu.Lock()
		recibidos = append(recibidos, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer servidor.Close()

	h := newHarness(t)
	args := h.args()
	args.StatusEndpoint = servidor.URL
	args.ExecutionID = "11111111-2222-3333-4444-555555555555"
	args.InputFile = h.writeRequest(h.request())

	result := h.execute(args, nil)
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	mu.Lock()
	defer mu.Unlock()

	// Hasta la spec 19 éste era el ÚNICO POST, porque nadie emitía etapas. Ahora
	// hay uno por step además del terminal, que es precisamente lo que §5.4 compra:
	// el portal deja de ver una ejecución muda hasta que termina.
	require.Len(t, recibidos, 3)
	assert.Equal(t, "running_step:01-test", recibidos[0]["current_stage"])
	assert.Equal(t, "running_step:02-supply", recibidos[1]["current_stage"])

	// Y el terminal sigue siendo el último y sigue llevando lo suyo. Es la
	// corrección a I-6 (spec 16) puesta a prueba desde el otro lado: cablear las
	// etapas no puede costar la única señal de que el contenedor terminó.
	terminal := recibidos[len(recibidos)-1]
	assert.Equal(t, args.ExecutionID, terminal["execution_id"])
	assert.Equal(t, "succeeded", terminal["status"])
	assert.Equal(t, float64(cli.ExitSucceeded), terminal["exit_code"])
}

// ── Spec 21: lo registrado llega al destino ─────────────────────────────────

// §3, y es la spec entera en un caso: hasta aquí el motor producía un registro
// perfecto que moría con el área de trabajo. En modo remoto eso significa una Fly
// Machine con `auto_destroy` que se lleva el registro al terminar; en local, un
// `staging/` que vive fuera del volumen precisamente para no depender de él.
func TestRunCommand_LoRegistradoLlegaAlDestino(t *testing.T) {
	h := newHarness(t)

	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	// El objeto llega ENTERO —su material, no su hash— porque es lo que un
	// tercero tiene que poder recomponer para verificar la identidad.
	objetos := h.objetosDelDestino()
	require.Len(t, objetos, 1)
	assert.Equal(t, h.elObjeto().ContentID, objetos[0].ContentID)
	assert.Equal(t, h.elObjeto().Canonical, objetos[0].Canonical)

	// Y la tira llega COMPLETA, incluido el último hecho: `attempt_finished` lo
	// emite el caso de uso después de la cadena, así que sólo el empuje de cierre
	// puede llevarlo. Si estuviera, el empuje por step no bastaría.
	enDestino := h.hechosDelDestino()
	enStaging := h.hechos()
	require.NotEmpty(t, enStaging)
	assert.Equal(t, len(enStaging), len(enDestino),
		"el empuje no filtra: transporta")

	tipos := make([]string, 0, len(enDestino))
	for _, hecho := range enDestino {
		tipos = append(tipos, hecho.Type)
	}
	assert.Equal(t, record.TypeAttemptStarted.String(), tipos[0])
	assert.Equal(t, record.TypeAttemptFinished.String(), tipos[len(tipos)-1],
		"el empuje de cierre es el que se lleva el desenlace")
}

// §5.1 y §5.5 hechos observables a la vez: lo que se empuja es ENUMERABLE.
// `objects/` y `events/` aparecen en el destino porque el empuje los crea;
// `keys/` está allí y no se empuja nunca —lo pone el cableado—, y `.clones/` y
// `.copias/` no están porque no son del registro.
func TestRunCommand_LoQueSeSincronizaEsEnumerable(t *testing.T) {
	h := newHarness(t)

	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	assert.Equal(t,
		[]string{"cache", "events", "keys", "lineage", "objects", "state"},
		h.directoriosDelDestino())
}

// §5.3: el `ack` existe, apunta a lo último confirmado, y NO se avanza antes de
// tiempo. Su valor tiene que ser el del último hecho de la tira, porque el
// empuje de cierre confirma hasta ahí.
func TestRunCommand_ElAckApuntaALoUltimoConfirmadoPorElDestino(t *testing.T) {
	h := newHarness(t)

	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	ack, existe := h.ackDelDestino()
	require.True(t, existe, "el empuje dejó su puntero")

	tira := h.ultimaTira()
	require.NotEmpty(t, tira)
	assert.Equal(t, tira[len(tira)-1].Seq, ack.Seq)

	// Vive en el ÁREA DE TRABAJO, fuera del volumen: perderlo degrada a reenvío
	// total, así que hacerlo depender del volumen al que optimiza sería circular.
	assert.NoDirExists(t, filepath.Join(h.destino, "ack"))
}

// §5.5 y §7: un destino que siempre falla NO cambia el exit code del pipeline,
// y el hueco queda explicado en vez de descubrirse por casualidad.
func TestRunCommand_UnDestinoQueFallaNoCambiaElResultadoYDejaElHuecoExplicado(t *testing.T) {
	h := newHarness(t)
	h.bloquearElDestino()

	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
	assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps(),
		"el registro no puede hacer fallar lo que observa")

	// Un `sync_failed` por EMPUJE, y los empujes son «uno por step más el del
	// cierre» (§7): con los dos steps del fixture, tres. Ni uno por hecho —que es
	// lo que la opción A costaba— ni uno solo al final.
	fallos := deTipo(h.ultimaTira(), record.TypeSyncFailed.String())
	require.Len(t, fallos, 3, "dos steps y el cierre: n+1 empujes, no n·hechos y no 1")
	assert.Contains(t, texto(fallos[0].Payload, "destination"), "local:"+h.destino)
	assert.NotEmpty(t, texto(fallos[0].Payload, "cause"))

	// El del cierre va DESPUÉS del desenlace, y es el único que no puede llegar
	// al destino: si el último empuje falló, nada emitido después de él puede
	// llegar por definición. Los dos anteriores sí habrían viajado con el empuje
	// siguiente, porque el `ack` no avanzó.
	tira := h.ultimaTira()
	assert.Equal(t, record.TypeSyncFailed.String(), tira[len(tira)-1].Type)
	assert.Equal(t, record.TypeAttemptFinished.String(), tira[len(tira)-2].Type)

	// Y lo escrito localmente sigue íntegro: registrar es incondicional, empujar
	// es lo que la spec 21 añade encima.
	assert.NotEmpty(t, h.hechos())
	assert.Len(t, h.objetos(), 1)
	assert.Empty(t, h.hechosDelDestino())
}

// §7, «recuperación»: con un destino que falla en un step y funciona en el
// siguiente, el destino final contiene TODOS los hechos, incluidos los del step
// que falló. No hay código de recuperación: hay ausencia de checkpoint. El `ack`
// no avanzó, así que la selección siguiente arranca donde arrancaba la anterior.
func TestRunCommand_ElEmpujeSiguienteRecogeLoPendienteDelQueFallo(t *testing.T) {
	// El primer comando de 02-supply retira el bloqueo, así que el empuje del
	// step 01 falla y el del step 02 ya encuentra el destino sano.
	h := newHarness(t, withPipelineFile("steps/02-supply/commands.yaml",
		"- name: unblock\n"+
			"  description: retira el bloqueo del destino\n"+
			"  cmd: rm -f \"$VEX_TEST_UNBLOCK\"\n"+
			"- name: provision\n"+
			"  description: aprovisiona el registro de contenedores\n"+
			"  cmd: echo '02-supply acr_name = \"${var.registry_prefix}-${var.artifact_name}\"'"+
			" | tee -a \"$VEX_TEST_LOG\"\n"+
			"  outputs:\n"+
			"    - name: acr_name\n"+
			"      description: nombre del registro aprovisionado\n"+
			"      probe: acr_name = \"([^\"]+)\"\n"))

	h.bloquearElDestino()
	t.Setenv("VEX_TEST_UNBLOCK", h.bloqueoDelDestino())

	result := h.run()
	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	// El fallo quedó registrado…
	require.NotEmpty(t, deTipo(h.ultimaTira(), record.TypeSyncFailed.String()),
		"control: el primer empuje falló de verdad")

	// …y aun así el destino acabó con TODA la tira, la parte anterior al fallo
	// incluida. Es el `ack` que no se movió haciendo su trabajo.
	assert.Equal(t, len(h.hechos()), len(h.hechosDelDestino()))
	require.Len(t, h.objetosDelDestino(), 1, "y la intención llegó con el primer empuje que funcionó")

	posiciones := make([]uint64, 0, len(h.hechosDelDestino()))
	for _, hecho := range h.hechosDelDestino() {
		posiciones = append(posiciones, hecho.Seq)
	}
	assert.Equal(t, uint64(1), posiciones[0],
		"la tira del destino arranca en el primer hecho, no en el primero que se pudo mandar")
}
