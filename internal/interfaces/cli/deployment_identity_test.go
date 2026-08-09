package cli_test

// La identidad del despliegue, resuelta ANTES de ejecutar (spec 18).
//
// Los casos de aquí son la §7 de esa spec, y se agrupan aparte de
// `run_command_integration_test.go` por la misma razón que los de `staging_test`:
// hablan de una tienda nueva y de un eslabón nuevo de la cadena, no de lo que
// las specs anteriores ya fijaron. El montaje es el mismo harness.
//
// Lo que estos casos NO pueden probar en memoria —y por eso están aquí y no en
// `internal/domain/deployment`— es lo único que sólo se ve con disco: el fallo
// rápido, el objeto escrito antes del primer comando, y la identidad calculada
// sobre la fuente REALMENTE usada.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	infraDeployment "github.com/jairoprogramador/vex-engine/internal/infrastructure/deployment"
	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// ── Fallo rápido ────────────────────────────────────────────────────────────

// EL FALLO RÁPIDO, que es el beneficio que esta spec se cobra el mismo día.
//
// Hasta la spec 18 el `commands.yaml` de cada step se leía dentro de la cadena
// 2, justo antes de ejecutarlo, así que un typo en el del último step costaba
// todos los anteriores: tests corridos, infraestructura provisionada, artefacto
// construido. Es la misma forma de fallo que la spec 05 §9.1 midió y declaró
// como límite aceptado.
func TestRunCommand_UnCommandsYamlRotoFallaAntesDelPrimerStep(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/02-supply/commands.yaml", "esto: no es una lista\n"))

	result := h.run()

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "02-supply")
	assert.Empty(t, h.ranSteps(),
		"01-test no llega a correr: el material de TODA la operación se lee antes del primer comando")
	assert.Empty(t, h.objetos(),
		"sin material no hay identidad que componer, así que tampoco hay objeto")
}

// EL BORDE DEL PASO 1, fijado explícitamente para que sea una decisión visible y
// no un descubrimiento.
//
// Se carga `request.Steps()`, que es `steps[:pedido+1]`: los steps POSTERIORES
// al pedido quedan fuera. Es deliberado —no forman parte de la operación, y
// meterlos en el objeto haría que `deploy` y `supply` tuvieran identidades
// contaminadas la una por la otra— y tiene el precio que se ve aquí: un
// `03-notify` roto no se descubre al pedir `supply`.
//
// El reparto que queda escrito es de ALCANCE, no de mecanismo: lo que hay que
// ver sobre el repositorio ENTERO lo ve el validador de la spec 04, que recibe
// todos los directorios de `steps/` antes del recorte.
func TestRunCommand_UnStepPosteriorAlPedidoNoEntraEnLaOperacion(t *testing.T) {
	t.Run("su contenido no se mira", func(t *testing.T) {
		h := newHarness(t,
			withPipelineFile("steps/03-notify/commands.yaml", "esto: no es una lista\n"))

		result := h.run()

		require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)
		assert.Equal(t, []string{"01-test", "02-supply"}, h.ranSteps())

		objeto := h.elObjeto()
		assert.Equal(t, []string{"01-test", "02-supply"}, stepsDe(objeto),
			"el objeto lleva los steps de la operación, no los del repositorio")
	})

	t.Run("su ESTRUCTURA sí, porque ésa la ve el validador de la spec 04", func(t *testing.T) {
		h := newHarness(t,
			withPipelineFile("steps/3-notify/commands.yaml", "- name: avisa\n  cmd: echo ok\n"))

		result := h.run()

		assert.Equal(t, cli.ExitFailed, result.exitCode)
		assert.Contains(t, result.stderr, "3-notify")
		assert.Empty(t, h.ranSteps())
	})
}

// ── La identidad existe antes de ejecutar ───────────────────────────────────

// LA AFIRMACIÓN CENTRAL: el objeto está escrito y el intento abierto antes de
// que corra el primer comando.
//
// Se observa haciendo fallar el PRIMER comando del PRIMER step: si el objeto
// existiera sólo al final, aquí no habría ninguno. Es la versión ejecutable de
// «verificable matando el proceso justo después del resolutor».
func TestRunCommand_LaIdentidadExisteAunqueElPrimerComandoFalle(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/01-test/commands.yaml",
		"- name: build\n  cmd: exit 7\n"))

	result := h.run()
	require.Equal(t, cli.ExitFailed, result.exitCode)

	objeto := h.elObjeto()
	assert.NotEmpty(t, objeto.ContentID)
	assert.Equal(t, "supply", objeto.Operation)
	assert.Equal(t, fixtureEnvironment, objeto.Destination)
	assert.Equal(t, h.projectURL, objeto.Subject)

	abiertos := h.hechosDeTipo(record.TypeAttemptStarted.String())
	require.Len(t, abiertos, 1, "un intento se abre una vez")
	assert.Equal(t, uint64(1), abiertos[0].Seq)
	assert.Equal(t, 1, abiertos[0].Attempt)
	assert.Equal(t, h.cabezaDelLinaje(fixtureEnvironment), abiertos[0].Payload["deployment_id"],
		"el hecho de apertura es el único que lleva el deployment_id, y es el que se acaba de derivar")
}

// El objeto lleva el material de cada step, y lo lleva por su DECLARACIÓN.
//
// Dos ejecuciones que resuelven valores distintos para la misma declaración
// producen el mismo `content_id`; es lo que permite conocer la identidad antes
// de ejecutar, porque un valor de runtime no se puede saber por adelantado y la
// declaración de cómo se obtiene sí.
func TestRunCommand_ElObjetoLlevaLaDeclaracionDeCadaStep(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	objeto := h.elObjeto()
	require.Len(t, objeto.Steps, 2)

	supply := objeto.Steps[1]
	assert.Equal(t, "02-supply", supply.StepID)
	assert.Equal(t, "environment", supply.Scope)
	assert.Contains(t, supply.Instructions, "inst-v1:")
	require.Len(t, supply.Parameters, 1)
	assert.Equal(t, "registry_prefix", supply.Parameters[0].Name)
	assert.Contains(t, supply.Parameters[0].Declaration, "vexsand",
		"un literal entra entero: dos pipelinecode que sólo difieren en su valor declaran cosas distintas")

	assert.Contains(t, objeto.Source.Project, "v1:")
	assert.Contains(t, objeto.Source.Pipeline, "v1:")
	assert.NotEqual(t, objeto.Source.Project, objeto.Source.Pipeline,
		"son dos árboles distintos leídos con la misma regla")
	assert.NotEmpty(t, objeto.Canonical,
		"la forma canónica viaja entera: es lo que un tercero tiene que poder reproducir bit a bit")
}

// ── Determinismo y sensibilidad ─────────────────────────────────────────────

// DETERMINISMO: dos ejecuciones del mismo árbol, mismo pipeline y mismo ambiente
// producen el mismo `content_id` — y por tanto UN objeto, porque la tienda está
// direccionada por contenido y es write-once.
//
// La POSICIÓN, en cambio, avanza: el mismo contenido dos veces cae en dos
// `deployment_id` distintos, porque el segundo cuelga del primero. No es un
// defecto, es la diferencia entre las dos identidades.
func TestRunCommand_ElMismoContenidoDaLaMismaIdentidadYOtraPosicion(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	primeraCabeza := h.cabezaDelLinaje(fixtureEnvironment)
	require.NotEmpty(t, primeraCabeza)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	assert.Len(t, h.objetos(), 1,
		"el mismo contenido no se escribe dos veces: la dirección ES el contenido")
	assert.NotEqual(t, primeraCabeza, h.cabezaDelLinaje(fixtureEnvironment),
		"la historia del ambiente avanza aunque la intención se repita")

	abiertos := h.hechosDeTipo(record.TypeAttemptStarted.String())
	require.Len(t, abiertos, 2, "dos intentos, dos aperturas")
	assert.NotEqual(t, abiertos[0].Payload["deployment_id"], abiertos[1].Payload["deployment_id"])
}

// Un byte del proyecto cambia la identidad. Es la mitad que hace útil al
// identificador: si no cambiara, «esta misma configuración, ¿ya corrió?» no
// distinguiría dos códigos distintos.
func TestRunCommand_UnByteDelProyectoCambiaLaIdentidad(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	h.writeProjectFile("src/app.txt", "v2\n")
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	objetos := h.objetos()
	require.Len(t, objetos, 2)
	assert.NotEqual(t, objetos[0].Source.Project, objetos[1].Source.Project)
}

// LA HUELLA DEL PIPELINECODE ENTRA EN LA IDENTIDAD, y hasta la spec 18 no se
// capturaba nada del pipeline.
//
// Se cambia un archivo que NO es material de ningún step —un README— para que lo
// único que se mueva sea la huella del ÁRBOL: con un `commands.yaml` cambiaría
// también la huella de instrucciones y el caso no distinguiría cuál de las dos
// hizo el trabajo.
func TestRunCommand_UnArchivoDelPipelinecodeCambiaLaIdentidad(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	primero := h.elObjeto()

	h.commitPipelineFile("README.md", "# pipelinecode de prueba\n")
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	objetos := h.objetos()
	require.Len(t, objetos, 2)

	otro := objetos[0]
	if otro.ContentID == primero.ContentID {
		otro = objetos[1]
	}
	assert.NotEqual(t, primero.Source.Pipeline, otro.Source.Pipeline)
	assert.Equal(t, primero.Source.Project, otro.Source.Project,
		"lo que cambió es el pipelinecode; la huella del proyecto no se mueve")
	assert.Equal(t, stepsDe(primero), stepsDe(otro),
		"tampoco cambia el material de los steps: el README no es de ninguno")
}

// EL COMMIT NO IDENTIFICA. Dos clones del mismo árbol con distinto commit —un
// rebase, un cherry-pick, dos remotos distintos— producen el MISMO `content_id`.
//
// Sin esto la comparabilidad entre organizaciones desaparece: la misma
// configuración desplegada desde dos forks sería dos cosas distintas. La huella
// identifica; el commit documenta, y por eso viaja como metadato del objeto.
func TestRunCommand_ElCommitNoIdentifica(t *testing.T) {
	h := newHarness(t)
	otro := h.conOtroPipeline()

	require.NotEqual(t, h.pipelineHead(), otro.pipelineHead(),
		"dos repos con el mismo árbol y distinto mensaje de commit: mismos bytes, otro HEAD")

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	require.Equal(t, cli.ExitSucceeded, otro.run().exitCode)

	objeto := h.elObjeto()
	assert.Equal(t, h.pipelineHead(), objeto.Source.PipelineCommit,
		"el objeto es write-once: queda el metadato del primero que lo declaró")
	assert.NotEmpty(t, objeto.Source.ProjectCommit)
}

// ── La ventana de reutilización del clon ────────────────────────────────────

// DENTRO DE LA VENTANA NO SE RE-CLONA. Es comportamiento NUEVO: hasta la spec 18
// el clonador hacía `os.RemoveAll` + clon en cada ejecución.
//
// Se observa por la identidad y no contando llamadas: si el clon se hubiera
// refrescado, el README nuevo movería la huella del árbol del pipelinecode y
// habría dos objetos. Con el clon reutilizado hay uno, y eso dice a la vez que
// no se re-clonó y que **la identidad se calculó sobre la fuente realmente
// usada**.
func TestRunCommand_LaVentanaEvitaVolverAClonar(t *testing.T) {
	// Sin `clone_window`: la ventana por defecto, 24 h.
	h := newHarness(t, withPipelineFile("vexpipeline.yaml", "schema_version: 1\n"))

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	h.commitPipelineFile("README.md", "# cambiado en el remoto\n")

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	assert.Len(t, h.objetos(), 1, "dentro de la ventana el remoto ni se mira")

	// Fuera de la ventana sí.
	h.envejecerElClon(25 * time.Hour)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	assert.Len(t, h.objetos(), 2, "pasada la ventana se vuelve a clonar y el cambio se ve")
}

// EL FALLBACK: con el remoto inaccesible y un clon en disco, la ejecución
// PROCEDE en vez de fallar, y el hecho queda registrado. **Hoy falla.**
//
// Las dos mitades importan y la segunda más: la identidad se calcula sobre la
// fuente realmente usada, así que sin `stale_clone_used` un `content_id`
// afirmaría que se ejecutó una versión del pipeline distinta de la que corrió y
// nadie podría saberlo.
func TestRunCommand_ConElRemotoCaidoSeUsaElClonViejoYSeRegistra(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	primero := h.elObjeto()

	// El remoto cambia y deja de responder. El clon local es lo único que hay.
	h.commitPipelineFile("README.md", "# cambiado en el remoto\n")
	h.envejecerElClon(3 * time.Hour)
	h.elRemotoNoResponde()

	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	viejos := h.hechosDeTipo(record.TypeStaleCloneUsed.String())
	require.Len(t, viejos, 1)
	assert.Equal(t, h.pipelineURL, viejos[0].Payload["source"])
	assert.InDelta(t, 3.0, viejos[0].Payload["age_hours"], 0.1)
	assert.Equal(t, uint64(1), viejos[0].Seq,
		"el clon viejo se eligió ANTES de que el intento tuviera identidad: es el primer hecho de su tira, "+
			"por delante del `attempt_started` que lo permitió escribir")

	assert.Len(t, h.objetos(), 1)
	assert.Equal(t, primero.Source.Pipeline, h.elObjeto().Source.Pipeline,
		"la identidad es la del clon que se usó, no la del remoto que no se pudo traer")
}

// Sin clon al que caer, el remoto caído sigue siendo un fallo: no hay nada con
// lo que ejecutar y decirlo es todo lo que se puede hacer.
func TestRunCommand_SinClonAlQueCaerElRemotoCaidoFalla(t *testing.T) {
	h := newHarness(t)
	h.elRemotoNoResponde()

	result := h.run()

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "clonar")
	assert.Empty(t, h.ranSteps())
}

// ── El pipelinecode que no declara su formato ───────────────────────────────

// UN PIPELINECODE SIN `vexpipeline.yaml` produce objeto MARCADO como incompleto,
// no ausencia de objeto (§5.5).
//
// Es la disciplina de siempre: *guardar hechos, nunca conclusiones*. «Este
// material está incompleto» es un hecho —el pipelinecode no declara de dónde
// salen sus variables—; omitir el objeto sería perder la historia, y emitirlo
// como completo sería mentir.
func TestRunCommand_UnPipelinecodeSinManifiestoProduceUnObjetoIncompleto(t *testing.T) {
	h := newHarness(t, withoutPipelineFile("vexpipeline.yaml"))

	result := h.run()

	require.Equal(t, cli.ExitSucceeded, result.exitCode, result.stderr)

	objeto := h.elObjeto()
	assert.False(t, objeto.Complete)
	assert.False(t, objeto.Format.Declared)
	assert.Equal(t, 1, objeto.Format.SchemaVersion)
}

// Y el día que ese mismo pipelinecode gane su `vexpipeline.yaml`, su
// `content_id` CAMBIA aunque no se toque ninguna otra línea.
//
// Es correcto —el mismo material leído bajo otras reglas significa otra cosa— y
// hay que tenerlo escrito: cada despliegue emitido antes de migrar es una
// identidad permanente incomparable con las de después.
func TestRunCommand_DeclararElFormatoCambiaLaIdentidad(t *testing.T) {
	h := newHarness(t, withoutPipelineFile("vexpipeline.yaml"))

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	incompleto := h.elObjeto()
	require.False(t, incompleto.Complete)

	h.commitPipelineFile("vexpipeline.yaml", "schema_version: 1\n"+ventanaDelFixture)
	// Sin manifiesto la ventana es la de por defecto, así que hay que salir de
	// ella para que el clon se refresque y el archivo nuevo se vea.
	h.envejecerElClon(25 * time.Hour)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	objetos := h.objetos()
	require.Len(t, objetos, 2)
	assert.NotEqual(t, objetos[0].Complete, objetos[1].Complete)
}

// ── La poda de la copia de trabajo ──────────────────────────────────────────

// UN ARCHIVO RETIRADO DEL PIPELINECODE NO SOBREVIVE EN EL WORKDIR.
//
// `Copy` sobrescribía y nunca podaba, así que un archivo que ya no existe en el
// pipelinecode se quedaba en la copia de trabajo con su último contenido
// interpolado. Hasta la spec 18 el clonador tapaba la mitad del problema
// borrando el clon en cada corrida; con la ventana de reutilización esa red
// desaparece y el residuo pasa a cruzar ejecuciones (§5.4).
//
// Lo que NO se poda es lo que los comandos generan: eso no es residuo, es
// trabajo. Por eso se poda lo que la copia ANTERIOR puso y ésta ya no pone, y no
// «todo lo que no está en el origen».
func TestRunCommand_UnArchivoRetiradoDelPipelinecodeSePodaDelWorkdir(t *testing.T) {
	h := newHarness(t, withPipelineFile("steps/02-supply/k8s/deployment.yaml", "replicas: 1\n"))

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	require.True(t, h.workdirTiene("steps/02-supply/k8s/deployment.yaml"))

	// Un archivo que los comandos generan dentro del workdir, no la copia.
	h.escribirEnElWorkdir("steps/02-supply/k8s/.terraform.lock", "generado")

	h.borrarPipelineFile("steps/02-supply/k8s/deployment.yaml")
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	assert.False(t, h.workdirTiene("steps/02-supply/k8s/deployment.yaml"),
		"lo que el pipelinecode ya no trae no se queda con su último contenido interpolado")
	assert.True(t, h.workdirTiene("steps/02-supply/k8s/.terraform.lock"),
		"lo que generó un comando no es residuo de la copia: podarlo sería una pérdida")
}

// stepsDe son los identificadores de los steps que el objeto lleva dentro.
func stepsDe(objeto infraDeployment.FileObjectDTO) []string {
	ids := make([]string, 0, len(objeto.Steps))
	for _, step := range objeto.Steps {
		ids = append(ids, step.StepID)
	}
	return ids
}
