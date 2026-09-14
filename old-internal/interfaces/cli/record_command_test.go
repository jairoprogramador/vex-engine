package cli_test

// La consulta y el mantenimiento del registro (spec 22 §7).
//
// Estos casos son la razón de que la spec no se difiera: comprueban que lo
// recolectado SIRVE, con el mismo `Fold` que ejecutará el consumidor real, mientras
// todavía hay pocos datos permanentes emitidos. Un defecto en el vocabulario o en el
// pliegue descubierto meses después se descubre con los datos ya escritos, y los
// objetos y los eventos no se corrigen borrando.
//
// El montaje es el harness de la spec 01: se ejecuta el motor de verdad y luego se le
// pregunta por lo que escribió. Preguntarle a hechos fabricados a mano probaría que la
// consulta se entiende consigo misma.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	infraDeployment "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/deployment"
	infraRecord "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/record"
	"github.com/jairoprogramador/vex-engine/old-internal/interfaces/cli"
)

// ── Reconstrucción ──────────────────────────────────────────────────────────

// LA PRUEBA CENTRAL DE LA SPEC: `record show` reconstruye lo que se observó.
//
// «Reconstruye» significa dos cosas y las dos importan: la intención congelada
// —el objeto con su `content_id`— y el resultado del intento. Son las dos preguntas
// que un despliegue permanente tiene que poder contestar por separado.
func TestRecord_ShowReconstruyeElObjetoYElIntento(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	despliegue := h.cabezaDelLinaje(fixtureEnvironment)
	require.NotEmpty(t, despliegue)
	salida := h.recordShow(despliegue)

	// El objeto: lo que se pretendía hacer.
	assert.Contains(t, salida, h.elObjeto().ContentID)
	assert.Contains(t, salida, h.projectURL)
	assert.Contains(t, salida, "ambiente:    "+fixtureEnvironment)
	assert.Contains(t, salida, "step 01-test")
	assert.Contains(t, salida, "step 02-supply")

	// El intento: lo que pasó. Se compara contra el pliegue de los hechos leídos por
	// el harness, que es la otra vía de llegar al mismo sitio.
	observado := record.Fold(h.eventos())
	assert.Contains(t, salida, "estado:      "+observado.Status.String())
	assert.Contains(t, salida, "último step: "+observado.LastStep)
	assert.Contains(t, salida, "step 02-supply: "+observado.Steps[1].Status.String())
}

// EL PLIEGUE DE LA PROYECCIÓN ES EL MISMO QUE EL DEL HARNESS, campo a campo.
//
// Es lo que hace de esta implementación la REFERENCIA contra la que validar el
// pliegue del backend (spec 26): si la proyección derivara algo distinto de lo que
// `Fold` deriva, lo que estaría mal sería la proyección — y esto lo detecta.
func TestRecord_LaProyeccionPliegaLoMismoQueSeObservo(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	observado := record.Fold(h.eventos())
	proyectado := h.recordAttempt(h.cabezaDelLinaje(fixtureEnvironment), 1)

	assert.Equal(t, observado.Status, proyectado.Status)
	assert.Equal(t, observado.Deployment, proyectado.Deployment)
	assert.Equal(t, observado.Attempt, proyectado.Attempt)
	assert.Equal(t, observado.LastStep, proyectado.LastStep)
	assert.Equal(t, observado.Commands, proyectado.Commands)
	assert.Equal(t, observado.FailedCommands, proyectado.FailedCommands)
	assert.Equal(t, observado.Steps, proyectado.Steps)
	assert.True(t, observado.StartedAt.Equal(proyectado.StartedAt))
}

// EL OBJETO DEL SEGUNDO DESPLIEGUE TAMBIÉN SE ENCUENTRA, y hace falta un caso propio
// porque una sola corrida no lo detecta.
//
// `deployment_id = H(content_id, parent)` y **todo despliegue menos el primero de su
// ambiente cuelga del anterior**, así que el enlace sólo se puede derivar si el padre
// está entre los candidatos. Buscar con una lista de un solo elemento —el pedido—
// deja los candidatos en «la raíz y él mismo» y no acierta nunca a partir del
// segundo. El fallo es silencioso: «no consta el objeto» es una respuesta legítima.
func TestRecord_ShowEncuentraElObjetoDeUnDespliegueQueNoEsElPrimero(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	primero := h.cabezaDelLinaje(fixtureEnvironment)

	h.writeProjectFile("src/app.txt", "v2\n")
	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	segundo := h.cabezaDelLinaje(fixtureEnvironment)
	require.NotEqual(t, primero, segundo, "el linaje avanzó")

	for _, despliegue := range []string{primero, segundo} {
		salida := h.recordShow(despliegue)
		assert.NotContains(t, salida, "no consta el objeto",
			"el enlace se deriva con el padre entre los candidatos: %s", despliegue)
		assert.Contains(t, salida, "content_id:  cnt-v1:")
	}
}

// DOS CONSULTAS SOBRE LOS MISMOS HECHOS DAN EL MISMO RESULTADO.
//
// Es la propiedad que hace utilizable una proyección derivada: si el orden de
// `os.ReadDir` o el de un mapa se colara en la salida, un diagnóstico no sería
// comparable con el de ayer.
func TestRecord_ShowEsDeterminista(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	despliegue := h.cabezaDelLinaje(fixtureEnvironment)

	assert.Equal(t, h.recordShow(despliegue), h.recordShow(despliegue))
}

// `record log` IMPRIME LOS HECHOS EN ORDEN DE `seq`, y los diez tipos que haya.
func TestRecord_LogImprimeLosHechosEnOrdenDeSeq(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	salida := h.recordLog(h.cabezaDelLinaje(fixtureEnvironment))

	for _, tipo := range []string{
		"attempt_started", "step_started", "command_started",
		"command_finished", "step_finished", "attempt_finished",
	} {
		assert.Contains(t, salida, tipo)
	}

	assert.Less(t, strings.Index(salida, "attempt_started"),
		strings.Index(salida, "attempt_finished"),
		"el orden es el de `seq`, y el desenlace va detrás de la apertura")
}

// EL INTENTO INTERRUMPIDO, que es la propiedad que justificó el event sourcing.
//
// No se puede matar el proceso de test, así que se monta la MISMA forma que deja una
// muerte dura: la tira escrita hasta ahí, sin el desenlace. Que `record show` devuelva
// `interrupted` CON EL ÚLTIMO STEP ALCANZADO —y no un error— es lo que hace que una
// Fly Machine que se va a mitad deje datos aprovechables.
func TestRecord_ShowDeUnIntentoInterrumpidoDiceHastaDondeLlego(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	h.truncarLaTira("attempt_finished")

	salida := h.recordShow(h.cabezaDelLinaje(fixtureEnvironment))

	assert.Contains(t, salida, "estado:      "+record.AttemptInterrupted.String())
	assert.Contains(t, salida, "último step: 02-supply")
	assert.Contains(t, salida, "destino válido de rollback: false",
		"de un intento interrumpido no se sabe si hizo lo que dice")
}

// PREGUNTAR POR UN DESPLIEGUE QUE NO EXISTE NO ES LO MISMO que preguntar por uno
// interrumpido, y el centinela los separa.
//
// `Fold(nil)` pliega a `interrupted`, así que devolver eso ante un identificador mal
// escrito convertiría un typo en un diagnóstico de interrupción.
func TestRecord_UnDespliegueQueNoConstaNoSeDisfrazaDeInterrumpido(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	inventado := "dep-v1:" + strings.Repeat("ef", 32)
	err := h.recordCommand(cli.RecordArgs{}).Show(
		context.Background(), &bytes.Buffer{}, cli.RecordArgs{}, inventado)

	require.Error(t, err)
	assert.ErrorIs(t, err, record.ErrAttemptNoConsta)
}

// LAS DOS RAÍCES NO SON EQUIVALENTES, y el área de trabajo es el SUPERCONJUNTO.
//
// Con el destino inservible el intento ocurre igual —registrar es incondicional— y no
// se publica. «¿Qué ocurrió aquí?» y «¿qué se publicó?» son dos preguntas, y este caso
// es la diferencia hecha observable.
func TestRecord_ElAreaDeTrabajoEsUnSuperconjuntoDelDestino(t *testing.T) {
	h := newHarness(t)
	h.bloquearElDestino()

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	despliegue := h.cabezaDelLinaje(fixtureEnvironment)
	require.NotEmpty(t, despliegue)

	assert.Contains(t, h.recordShow(despliegue), "estado:      succeeded",
		"el área de trabajo tiene el intento: registrar es incondicional")

	err := h.recordCommand(cli.RecordArgs{From: cli.FromDestination}).Show(
		context.Background(), &bytes.Buffer{},
		cli.RecordArgs{From: cli.FromDestination}, despliegue)
	require.Error(t, err)
	assert.ErrorIs(t, err, record.ErrAttemptNoConsta,
		"y el destino no: el empuje no llegó")
}

// SIN EL VOLUMEN MONTADO, `--from work` SIGUE CONTESTANDO.
//
// Es el caso que da sentido al default, y exigir el destino al cablear lo rompía
// justamente aquí: en la máquina cuyo empuje no llegó, que es la que hay que poder
// mirar. Lo que sí necesita el destino es lo que opera sobre él —`gc` y `rebuild`— y
// ésos lo piden cuando les toca.
func TestRecord_SinVolumenMontadoElAreaDeTrabajoSigueContestando(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	despliegue := h.cabezaDelLinaje(fixtureEnvironment)

	// El volumen desaparece DESPUÉS de la ejecución, que es como se ve un destino que
	// nunca se montó desde el punto de vista de la consulta.
	require.NoError(t, os.RemoveAll(h.destino))

	args := cli.RecordArgs{StateConfigFile: h.stateConfig, StagingDir: h.staging}
	cmd, err := cli.BuildRecordCommand(cli.EngineConfig{RootVexPath: h.root}, args)
	require.NoError(t, err, "cablear una CONSULTA no puede exigir el volumen")

	var out bytes.Buffer
	require.NoError(t, cmd.Show(context.Background(), &out, args, despliegue))
	assert.Contains(t, out.String(), "estado:      succeeded")

	report, err := cmd.Verify(context.Background(), &bytes.Buffer{}, args)
	require.NoError(t, err)
	assert.Zero(t, report.Corruptos)

	// Y lo que opera sobre el destino sí se niega, con el volumen en el mensaje.
	_, err = cmd.GC(context.Background(), &bytes.Buffer{}, args)
	require.Error(t, err)
	assert.ErrorIs(t, err, cli.ErrInputInvalido)
	assert.Contains(t, err.Error(), "volumen")
}

// `sync_failed` SÓLO SE VE EN `record log`, y es lo que la spec 21 §5.5 dejó a deber.
//
// `Fold` lo ignora a propósito —no cambia el resultado del intento— así que no aparece
// en ningún `AttemptResult`. Sin este comando, un hueco del destino sólo se podría
// explicar mirando los logs, que son descartables por diseño.
func TestRecord_UnHuecoDelDestinoSeExplicaEnElLog(t *testing.T) {
	h := newHarness(t)
	h.bloquearElDestino()

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	salida := h.recordLog(h.cabezaDelLinaje(fixtureEnvironment))

	assert.Contains(t, salida, "sync_failed")
	assert.Contains(t, salida, "local:"+h.destino)
	assert.NotContains(t, h.recordShow(h.cabezaDelLinaje(fixtureEnvironment)), "sync_failed",
		"el pliegue no lo cuenta: no cambia lo que el intento fue")
}

// ── record history ──────────────────────────────────────────────────────────

// EL LISTADO MARCA LOS DESTINOS VÁLIDOS, que es el motivo de que exista.
//
// Nombrar un destino de rollback (spec 28) sin saber antes si ese intento sirve deja
// al usuario eligiendo uno que falla después.
func TestRecord_HistoryMarcaLosDestinosValidos(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	// Un segundo despliegue del mismo ambiente: el linaje avanza, así que son dos
	// posiciones distintas del mismo contenido.
	h.writeProjectFile("src/app.txt", "v2\n")
	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	salida := h.recordHistory(fixtureEnvironment)

	assert.Contains(t, salida, "2 intento(s) contra '"+fixtureEnvironment+"'")
	assert.Equal(t, 2, strings.Count(salida, " * dep-v1:"),
		"los dos terminaron bien con todos sus steps cerrados")
}

// UN AMBIENTE SIN HISTORIA NO ES UN ERROR: es un ambiente al que no se ha desplegado.
func TestRecord_HistoryDeUnAmbienteSinHistoriaNoEsUnError(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	assert.Contains(t, h.recordHistory("prod"), "no tiene historia")
}

// ── record verify ───────────────────────────────────────────────────────────

// UN REGISTRO SANO NO TIENE NADA QUE DECIR. Es la línea base de los cuatro casos que
// vienen detrás.
func TestRecord_VerifyNoEncuentraNadaEnUnRegistroSano(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	report, salida := h.recordVerify()

	assert.Empty(t, report.Faults, salida)
	assert.Zero(t, report.Corruptos)
	assert.Positive(t, report.Objetos)
	assert.Positive(t, report.Tiras)
}

// LOS TRES QUE DEBEN FALLAR (§7), montados a mano sobre un registro real.
//
// A mano porque el motor no los produce: son exactamente las formas que este comando
// existe para detectar, y un motor que las emitiera sería el defecto.
func TestRecord_VerifyDetectaLoQueDebeDetectar(t *testing.T) {
	t.Run("un seq con hueco", func(t *testing.T) {
		h := newHarness(t)
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		h.borrarDeLaTira(func(sobre map[string]any) bool {
			return sobre["seq"].(float64) == 3
		})

		report, salida := h.recordVerify()

		assert.Positive(t, report.Corruptos, salida)
		assert.Contains(t, salida, record.FaultSeqGap.String())
	})

	t.Run("un step_started sin su step_finished en un intento terminado", func(t *testing.T) {
		h := newHarness(t)
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		h.borrarDeLaTira(func(sobre map[string]any) bool {
			return sobre["type"] == "step_finished" &&
				sobre["payload"].(map[string]any)["step_id"] == "02-supply"
		})

		report, salida := h.recordVerify()

		assert.Positive(t, report.Corruptos, salida)
		assert.Contains(t, salida, record.FaultParAbierto.String())
		assert.Contains(t, salida, "02-supply")
	})

	t.Run("un objeto cuyo prefijo de versión se borró", func(t *testing.T) {
		h := newHarness(t)
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		h.mutarElObjeto(func(dto *infraDeployment.FileObjectDTO) {
			dto.Source.Project = ""
		})

		report, salida := h.recordVerify()

		assert.Positive(t, report.Corruptos, salida)
		assert.Contains(t, salida, record.FaultObjetoMalformado.String())
		assert.NotContains(t, salida, record.FaultNoComparable.String(),
			"un token que no se puede leer no es un motor más nuevo: es un archivo roto")
	})

	t.Run("un content_id que no recomputa", func(t *testing.T) {
		h := newHarness(t)
		require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

		h.mutarElObjeto(func(dto *infraDeployment.FileObjectDTO) {
			dto.Canonical += "\ncontaminado"
		})

		report, salida := h.recordVerify()

		assert.Positive(t, report.Corruptos, salida)
		assert.Contains(t, salida, record.FaultContentIDNoRecomputa.String())
	})
}

// Y EL CUARTO, QUE DEBE FALLAR DE OTRA MANERA: un registro SANO que este binario no
// puede verificar de forma comparable.
//
// Mismo comando, dos diagnósticos distintos. Reportarlo como corrupción sería la peor
// forma de fallo posible para la comprobación que existe para detectar corrupción, y
// convertiría cada actualización del motor en una alarma masiva.
func TestRecord_VerifyNoAcusaAlRegistroPorUnaReglaQueNoCalcula(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	h.mutarElObjeto(func(dto *infraDeployment.FileObjectDTO) {
		// La huella del árbol calculada con una regla que este binario no tiene. Es la
		// forma en la que llega de verdad: un objeto emitido por un motor más nuevo.
		dto.Source.Project = "v2:" + strings.Repeat("11", 32)
	})

	report, salida := h.recordVerify()

	assert.NotEmpty(t, report.Faults)
	assert.Zero(t, report.Corruptos,
		"ninguno acusa al registro: es un límite de este binario")
	assert.Contains(t, salida, record.FaultNoComparable.String())
	assert.Contains(t, salida, "v2")
	assert.NotContains(t, salida, record.FaultContentIDNoRecomputa.String())
}

// UN DESTINO AL QUE UN EMPUJE ANTERIOR NO LLEGÓ NO ES UN REGISTRO CORRUPTO, y es la
// otra cara del enlace derivado.
//
// El objeto está en el destino y su tira también; lo que falta es la tira del PADRE,
// del que se deriva la posición. Sin el padre el enlace no se puede recomponer, así
// que `verify` avisa —es una tienda que hay que mirar— y **no** llama a eso
// corrupción: hacerlo sería señalar un registro sano, que es el fallo que este
// comando existe para no cometer.
func TestRecord_UnDestinoAlQueFaltoUnEmpujeNoEsUnRegistroCorrupto(t *testing.T) {
	h := newHarness(t)

	// Primer intento: el empuje falla, así que el destino no recibe ni su objeto ni
	// su tira. Es el hueco.
	h.bloquearElDestino()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	// El destino vuelve a funcionar y el segundo intento sí se publica. Su posición
	// cuelga del primero, que allí no consta.
	require.NoError(t, os.RemoveAll(h.bloqueoDelDestino()))
	h.writeProjectFile("src/app.txt", "v2\n")
	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	args := cli.RecordArgs{From: cli.FromDestination}
	var out bytes.Buffer
	report, err := h.recordCommand(args).Verify(context.Background(), &out, args)
	require.NoError(t, err)

	salida := out.String()
	assert.Contains(t, salida, record.FaultObjetoAusente.String(), salida)
	assert.Zero(t, report.Corruptos,
		"el objeto está y está sano: lo que falta es el padre del que deriva la posición")
	assert.Contains(t, salida, "límites de este binario")
}

// UNA LÍNEA ILEGIBLE SE REPORTA en vez de leerse en silencio. La tira se lee igual
// —es la regla de tolerancia— y eso es exactamente por lo que hay que contarla.
func TestRecord_VerifyReportaUnaLineaIlegible(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	h.romperUnaLineaDeLaTira()

	report, salida := h.recordVerify()

	assert.Positive(t, report.Corruptos, salida)
	assert.Contains(t, salida, record.FaultLineaIlegible.String())
}

// ── record gc ───────────────────────────────────────────────────────────────

// SIN `--apply` NO SE TOCA NADA, y es el default.
//
// Quien decide qué se poda tiene que poder ver antes qué se podaría — y en un destino
// compartido por dos máquinas, lo que vería podría no ser suyo.
func TestRecord_GCSinApplyNoBorraNada(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	entradas := h.cacheEntries()
	require.NotEmpty(t, entradas)

	plan, salida := h.recordGC(cli.RecordArgs{Cache: true, All: true})

	assert.False(t, plan.Aplicado)
	assert.Zero(t, plan.Borrados)
	assert.NotEmpty(t, plan.CachePodables)
	assert.Equal(t, entradas, h.cacheEntries())
	assert.Contains(t, salida, "nada se ha borrado")
}

// LAS REGLAS DE VIDA, HECHAS EJECUTABLES: el `gc` poda el índice ENTERO y no toca
// ninguna de las cuatro tiendas que no se borran nunca.
//
// Y borrar el índice entero no puede perder un hecho: la ejecución siguiente
// re-ejecuta todo y ninguna variable con efecto real desaparece. Si desapareciera
// alguna, la que estaría mal es la clasificación de la spec 11.
func TestRecord_GCPodaElIndiceYNoTocaLoQueNuncaSeBorra(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	registrosAntes := h.persistedStepState("02-supply")
	objetosAntes := h.objetosDelDestino()
	hechosAntes := h.hechosDelDestino()
	linajeAntes := h.cabezaDelLinaje(fixtureEnvironment)
	secretoAntes, err := os.ReadFile(h.rutaDelSecreto())
	require.NoError(t, err)
	require.NotEmpty(t, h.cacheEntries())

	plan, _ := h.recordGC(cli.RecordArgs{Cache: true, All: true, Apply: true})

	assert.Positive(t, plan.Borrados)
	assert.Empty(t, h.cacheEntries(), "el índice es derivable: entero es seguro")

	assert.Equal(t, registrosAntes, h.persistedStepState("02-supply"), "state/ no se toca")
	assert.Equal(t, objetosAntes, h.objetosDelDestino(), "objects/ no se toca")
	assert.Equal(t, hechosAntes, h.hechosDelDestino(), "events/ no se toca")
	assert.Equal(t, linajeAntes, h.cabezaDelLinaje(fixtureEnvironment), "lineage/ no se toca")

	secretoDespues, err := os.ReadFile(h.rutaDelSecreto())
	require.NoError(t, err)
	assert.Equal(t, secretoAntes, secretoDespues,
		"keys/ tampoco: borrarlo invalidaría en silencio todos los digests emitidos")

	// Y la comprobación que cierra la fila: tras el barrido, el motor sigue
	// decidiendo lo mismo que decidía.
	h.resetLog()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
}

// EL BÚFER SÍ SE PODA, y sólo lo que el destino confirmó.
//
// El criterio es comprobable y no una intuición: la tira está confirmada cuando el
// `seq` del `ack` iguala al mayor de su `.jsonl` (spec 21 §9).
func TestRecord_GCPodaLasTirasConfirmadasDelAreaDeTrabajo(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	ack, hayAck := h.ackDelDestino()
	require.True(t, hayAck, "el empuje funcionó, así que hay puntero")

	plan, _ := h.recordGC(cli.RecordArgs{Staging: true})
	require.Len(t, plan.Confirmadas, 1)
	require.Empty(t, plan.SinConfirmar)
	assert.Equal(t, ack.Seq, plan.Confirmadas[0].UltimoAck)
	assert.Equal(t, ack.Seq, plan.Confirmadas[0].MaxSeq)

	hechosDelDestino := h.hechosDelDestino()
	require.NotEmpty(t, hechosDelDestino)

	plan, _ = h.recordGC(cli.RecordArgs{Staging: true, Apply: true})

	assert.Positive(t, plan.Borrados)
	assert.Empty(t, h.hechos(), "la tira del área de trabajo se podó")
	assert.Equal(t, hechosDelDestino, h.hechosDelDestino(),
		"y la del destino sigue entera: es permanente")
}

// UNA TIRA SIN CONFIRMAR NO SE PODA, y la trampa está escrita: sin archivo de `ack` la
// tira no está «sin confirmar», está **NO CONSTA** — y las dos se podan igual de mal.
//
// De aquí sale además el trabajo que la spec 21 declaró suyo y no hizo: nadie empujará
// esta tira jamás, y recogerla exige el `Content` del objeto, que no se puede rehidratar
// sin perder información. El `gc` la LISTA, que es lo que hace posible el barrido
// cuando exista.
func TestRecord_GCNoPodaUnaTiraQueNadieConfirmo(t *testing.T) {
	h := newHarness(t)
	h.bloquearElDestino()
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	_, hayAck := h.ackDelDestino()
	require.False(t, hayAck, "el empuje falló, así que no hay puntero")

	plan, salida := h.recordGC(cli.RecordArgs{Staging: true, Apply: true})

	assert.Empty(t, plan.Confirmadas)
	require.Len(t, plan.SinConfirmar, 1)
	assert.False(t, plan.SinConfirmar[0].AckPresente)
	assert.Zero(t, plan.Borrados)
	assert.NotEmpty(t, h.hechos(), "la tira sigue ahí: borrarla sería un intento que no ocurrió")
	assert.Contains(t, salida, "NO CONSTA su ack")
	assert.Contains(t, salida, "barrido de arranque sigue pendiente")
}

// LOS HUÉRFANOS SE LISTAN Y NO SE BORRAN, y `state/` sólo se INVENTARÍA.
//
// Una tienda cuya regla es «no se borra nunca» no puede tener un comando que la borre,
// por conveniente que parezca. Y «huérfano» no es una propiedad del almacén: depende
// del pipelinecode —renumerar un step, retirar `rules`— que este comando no tiene.
func TestRecord_GCListaLosHuerfanosYNoLosBorra(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	// Un archivo del almacén VIEJO, que la spec 11 dejó en su sitio a propósito.
	huerfano := filepath.Join(
		h.root, cli.VexHomeDirName, "projects", "demo-app", "store", "pipeline", "shared",
		"supply.vars")
	writeFile(t, huerfano, "gob inerte")

	plan, salida := h.recordGC(cli.RecordArgs{Cache: true, All: true, Apply: true})

	assert.Contains(t, plan.VarsHuerfanos, huerfano)
	assert.FileExists(t, huerfano, "se LISTA, no se borra: la decisión es del operador")
	assert.NotEmpty(t, plan.Estado, "y de state/ se da el inventario")
	assert.Contains(t, salida, "NUNCA se borran")
	assert.Contains(t, salida, "02-supply")
	assert.Contains(t, salida, "store-vars",
		"las filas remotas del almacén viejo no se ven desde aquí, y se dice")
}

// ── record rebuild ──────────────────────────────────────────────────────────

// EL ÍNDICE SE RECONSTRUYE DESDE `state/`, entrada por entrada.
//
// Es lo que hace que `gc --cache --all` sea barato: lo que se borra se puede rehacer.
// La url del proyecto —que la ruta abrevia con un hash y no se puede invertir— sale de
// `lineage/` y `objects/`, que la llevan dentro.
func TestRecord_RebuildDevuelveElIndiceQueElGCBorro(t *testing.T) {
	h := newHarness(t)
	unaCorridaYPuntoFijo(t, h)

	entradasAntes := h.cacheEntries()
	require.NotEmpty(t, entradasAntes)

	h.borrarElIndice()
	require.Empty(t, h.cacheEntries())

	report, salida := h.recordRebuild()

	assert.Positive(t, report.Registros, salida)
	assert.Positive(t, report.Entradas)
	assert.Empty(t, report.SinSubject, "la url del proyecto se recuperó de lineage/")
	assert.Empty(t, report.Errores)
	assert.Equal(t, entradasAntes, h.cacheEntries(),
		"la entrada es {cache_key, state_key, record_id} y los tres salen del almacén")
}

// UN REGISTRO SIN HUELLA NO SE INDEXA, y no es un error.
//
// Es el step que se ejecutó sin poder componer su material: precisamente lo que no
// puede hacer es revivir (spec 11 §4). Indexarlo bajo una clave incompleta lo haría
// colisionar con cualquier otro al que le falte lo mismo.
func TestRecord_RebuildNoIndexaUnRegistroSinHuella(t *testing.T) {
	h := newHarness(t,
		withoutPipelineFile("steps/02-supply/config.yaml"),
		withoutPipelineFile("steps/01-test/config.yaml"))
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	h.borrarElIndice()
	report, _ := h.recordRebuild()

	assert.Zero(t, report.Entradas,
		"un step sin ámbito no persiste nada, así que no hay nada que indexar")
	assert.Zero(t, report.Registros)
}

// ── El extremo a extremo de la fase (§7) ────────────────────────────────────

// LAS TRES TIENDAS SE POBLARON Y LA CONSULTA LAS LEE.
//
// Es el cierre de la fase entera: `objects/`, `events/` y `cache/` tienen contenido tras
// una ejecución contra un pipelinecode real con `--state-config type: local`, y
// `record show` reconstruye desde ellas. Hasta esta spec, nada las leía.
func TestRecord_LaFaseCierraLasTresTiendasSePueblanYSeLeen(t *testing.T) {
	h := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	require.NotEmpty(t, h.objetos(), "objects/")
	require.NotEmpty(t, h.hechos(), "events/")
	require.NotEmpty(t, h.cacheEntries(), "cache/")

	report, salida := h.recordVerify()
	require.Zero(t, report.Corruptos, salida)

	reconstruido := h.recordShow(h.cabezaDelLinaje(fixtureEnvironment))
	assert.Contains(t, reconstruido, "content_id:  "+h.elObjeto().ContentID)
	assert.Contains(t, reconstruido, "estado:      succeeded")
}

// ── Andamiaje de la consulta ────────────────────────────────────────────────

// recordCommand cablea `vexd record` con las mismas rutas que el motor del harness.
//
// Es lo que hace que la consulta y la escritura no puedan mirar sitios distintos: si
// el comando derivara el destino o el área de trabajo por su cuenta, estos casos
// pasarían y en producción el `gc` barrería otro directorio.
func (h *harness) recordCommand(args cli.RecordArgs) *cli.RecordCommand {
	h.t.Helper()

	args.StateConfigFile = h.stateConfig
	args.StagingDir = h.staging

	cmd, err := cli.BuildRecordCommand(cli.EngineConfig{RootVexPath: h.root}, args)
	require.NoError(h.t, err)
	require.Equal(h.t, h.staging, cmd.StagingDir())
	return cmd
}

func (h *harness) recordShow(deploymentID string) string {
	h.t.Helper()

	var out bytes.Buffer
	args := cli.RecordArgs{StateConfigFile: h.stateConfig, StagingDir: h.staging}
	require.NoError(h.t, h.recordCommand(args).Show(
		context.Background(), &out, args, deploymentID))
	return out.String()
}

func (h *harness) recordLog(deploymentID string) string {
	h.t.Helper()

	var out bytes.Buffer
	args := cli.RecordArgs{StateConfigFile: h.stateConfig, StagingDir: h.staging}
	require.NoError(h.t, h.recordCommand(args).Log(
		context.Background(), &out, args, deploymentID))
	return out.String()
}

func (h *harness) recordHistory(environment string) string {
	h.t.Helper()

	var out bytes.Buffer
	args := cli.RecordArgs{StateConfigFile: h.stateConfig, StagingDir: h.staging}
	require.NoError(h.t, h.recordCommand(args).History(
		context.Background(), &out, args, environment))
	return out.String()
}

// recordAttempt pregunta por el PUERTO de lectura, que es lo que el consumidor real
// usará: el comando imprime, y lo que la spec 26 va a reimplementar es esto.
func (h *harness) recordAttempt(deploymentID string, numero int) record.AttemptResult {
	h.t.Helper()

	id, err := deployment.ParseDeploymentID(deploymentID)
	require.NoError(h.t, err)
	intento, err := deployment.NewAttempt(numero)
	require.NoError(h.t, err)

	proyeccion := infraRecord.NewScanProjection(
		filepath.Join(h.staging, "events"), filepath.Join(h.staging, "objects"))

	ctx := context.Background()
	resultado, err := proyeccion.Attempt(&ctx, id, intento)
	require.NoError(h.t, err)
	return resultado
}

func (h *harness) recordVerify() (cli.VerifyReport, string) {
	h.t.Helper()

	var out bytes.Buffer
	args := cli.RecordArgs{StateConfigFile: h.stateConfig, StagingDir: h.staging}
	report, err := h.recordCommand(args).Verify(context.Background(), &out, args)
	require.NoError(h.t, err)
	return report, out.String()
}

func (h *harness) recordGC(args cli.RecordArgs) (cli.GCPlan, string) {
	h.t.Helper()

	var out bytes.Buffer
	plan, err := h.recordCommand(args).GC(context.Background(), &out, args)
	require.NoError(h.t, err)
	return plan, out.String()
}

func (h *harness) recordRebuild() (cli.RebuildReport, string) {
	h.t.Helper()

	var out bytes.Buffer
	report, err := h.recordCommand(cli.RecordArgs{}).Rebuild(context.Background(), &out)
	require.NoError(h.t, err)
	return report, out.String()
}

// ── Deformaciones a mano de lo que el motor escribió ────────────────────────

// truncarLaTira quita del archivo todas las líneas de un tipo, que es la forma en la
// que una muerte dura deja la tira: lo escrito hasta ahí y nada más.
func (h *harness) truncarLaTira(tipo string) {
	h.t.Helper()
	h.borrarDeLaTira(func(sobre map[string]any) bool { return sobre["type"] == tipo })
}

// borrarDeLaTira reescribe la tira del área de trabajo sin las líneas que casen.
//
// Trabaja sobre el ARCHIVO y no sobre los eventos en memoria: lo que `record verify`
// lee es el archivo, y una deformación aplicada a otra cosa no probaría nada de lo que
// hace falta probar.
func (h *harness) borrarDeLaTira(casa func(map[string]any) bool) {
	h.t.Helper()

	path := h.rutaDeLaTira()
	data, err := os.ReadFile(path)
	require.NoError(h.t, err)

	conservadas := make([]string, 0, 16)
	for _, linea := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(linea) == "" {
			continue
		}
		var sobre map[string]any
		require.NoError(h.t, json.Unmarshal([]byte(linea), &sobre))
		if casa(sobre) {
			continue
		}
		conservadas = append(conservadas, linea)
	}
	require.NoError(h.t,
		os.WriteFile(path, []byte(strings.Join(conservadas, "\n")+"\n"), 0o644))
}

// romperUnaLineaDeLaTira corta una línea POR EL MEDIO, que es lo que deja una máquina
// que murió a mitad de un `write`.
func (h *harness) romperUnaLineaDeLaTira() {
	h.t.Helper()

	path := h.rutaDeLaTira()
	data, err := os.ReadFile(path)
	require.NoError(h.t, err)

	lineas := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Greater(h.t, len(lineas), 2)
	lineas[1] = lineas[1][:len(lineas[1])/2]

	require.NoError(h.t,
		os.WriteFile(path, []byte(strings.Join(lineas, "\n")+"\n"), 0o644))
}

func (h *harness) rutaDeLaTira() string {
	h.t.Helper()

	matches, err := filepath.Glob(filepath.Join(h.staging, "events", "*", "*", "*.jsonl"))
	require.NoError(h.t, err)
	require.Len(h.t, matches, 1, "el harness corre una ejecución por caso")
	return matches[0]
}

// mutarElObjeto reescribe el objeto del área de trabajo a mano.
//
// Por fuera del almacén a propósito: el almacén no deja escribir un objeto
// contradictorio —es write-once y compara la forma canónica— y lo que estos casos
// montan es precisamente el archivo que no debería existir.
func (h *harness) mutarElObjeto(mutar func(*infraDeployment.FileObjectDTO)) {
	h.t.Helper()

	matches, err := filepath.Glob(filepath.Join(h.staging, "objects", "*", "*", "*.json"))
	require.NoError(h.t, err)
	require.NotEmpty(h.t, matches)

	for _, path := range matches {
		data, err := os.ReadFile(path)
		require.NoError(h.t, err)

		var dto infraDeployment.FileObjectDTO
		require.NoError(h.t, json.Unmarshal(data, &dto))
		mutar(&dto)

		mutado, err := json.MarshalIndent(dto, "", "  ")
		require.NoError(h.t, err)
		require.NoError(h.t, os.WriteFile(path, mutado, 0o644))
	}
}
