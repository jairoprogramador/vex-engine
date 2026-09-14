package record_test

// Los invariantes del registro, hechos ejecutables (spec 22 §5.1').
//
// «`seq` monótono sin huecos, pares abiertos y cerrados» estaba escrito en prosa
// en la spec 17 y no había nada que lo comprobara. Estos casos son esa prosa
// convertida en código, y por eso viven en dominio: el pliegue del backend
// (spec 26) tiene que poder ejecutar exactamente estas reglas, y un invariante que
// sólo se puede comprobar con un disco delante no viaja.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
)

// UNA TIRA SANA NO TIENE NADA QUE DECIR. Es la línea base sin la que el resto de
// los casos no significan nada: si esto reportara algo, cualquier defecto que los
// demás encuentren podría ser el mismo falso positivo.
func TestCheckStrip_UnaTiraSanaNoTieneDefectos(t *testing.T) {
	eventos := tiraSana(t)

	assert.Empty(t, record.CheckStrip(eventos, record.Fold(eventos)))
}

// EL HUECO DE `seq`, que es el defecto que EXIGE recorrer la tira.
//
// `Fold` no lo ve y no es un descuido: ordena por posición y no le importa que
// falte una, porque plegar lo que hay es exactamente lo que tiene que hacer con un
// archivo truncado. De ahí que las dos mitades de la verificación tengan dos
// costes distintos y sólo una necesite la tira cruda (spec 17 §9).
func TestCheckStrip_UnHuecoEnSeqSeDetectaYFoldNoLoVe(t *testing.T) {
	eventos := tiraSana(t)
	conHueco := append([]record.Event{}, eventos[:2]...)
	conHueco = append(conHueco, eventos[3:]...)

	resultado := record.Fold(conHueco)
	require.Equal(t, record.AttemptSucceeded, resultado.Status,
		"el pliegue sigue dando un resultado: ignorar el hueco es su trabajo")

	faults := record.CheckStrip(conHueco, resultado)

	require.Len(t, faults, 1)
	assert.Equal(t, record.FaultSeqGap, faults[0].Kind)
	assert.True(t, faults[0].Kind.EsCorrupcion())
}

// EL HUECO POR DELANTE cuenta como hueco, y no como un caso propio.
//
// Es la forma real de una tira leída del destino a la que le faltan los primeros
// hechos: lo que falta es lo mismo, así que el vocabulario no crece.
func TestCheckStrip_UnaTiraQueNoEmpiezaEnUnoTambienTieneHueco(t *testing.T) {
	eventos := tiraSana(t)[1:]

	faults := record.CheckStrip(eventos, record.Fold(eventos))

	require.NotEmpty(t, faults)
	assert.Equal(t, record.FaultSeqGap, faults[0].Kind)
}

// LA MISMA POSICIÓN DOS VECES no la puede producir un emisor sano: dos hechos
// distintos afirmarían ocupar el mismo lugar.
func TestCheckStrip_LaMismaPosicionDosVecesEsUnDefecto(t *testing.T) {
	eventos := tiraSana(t)
	repetida := append([]record.Event{}, eventos...)
	repetida = append(repetida, evento(t, 2, 60, record.CommandStarted{
		StepID: "02-supply", CommandName: "otro",
	}))

	faults := record.CheckStrip(repetida, record.Fold(repetida))

	require.NotEmpty(t, faults)
	assert.Equal(t, record.FaultSeqRepetida, faults[0].Kind)
}

// UN PAR ABIERTO EN UN INTENTO QUE SÍ CERRÓ es un defecto, y sale GRATIS del
// pliegue: `Finished == false` ya distingue «empezó y no terminó» de «terminó
// mal», que son dos hechos y sólo uno es un fallo.
func TestCheckStrip_UnStepAbiertoEnUnIntentoTerminadoEsUnDefecto(t *testing.T) {
	eventos := []record.Event{
		evento(t, 1, 0, record.AttemptStarted{Deployment: despliegueDePrueba(t), Runner: "img"}),
		evento(t, 2, 1, record.StepStarted{StepID: "01-test"}),
		evento(t, 3, 2, record.AttemptFinished{Status: record.AttemptSucceeded}),
	}

	faults := record.CheckStrip(eventos, record.Fold(eventos))

	require.Len(t, faults, 1)
	assert.Equal(t, record.FaultParAbierto, faults[0].Kind)
	assert.Contains(t, faults[0].Detail, "01-test")
}

// Y EL MISMO PAR ABIERTO EN UN INTENTO INTERRUMPIDO NO LO ES.
//
// Es la asimetría que hace útil el modelo: un par sin cerrar ES lo que la
// interrupción significa, y acusarlo convertiría el caso normal de una máquina
// efímera en una alarma de corrupción.
func TestCheckStrip_UnIntentoInterrumpidoNoAcusaSusParesAbiertos(t *testing.T) {
	eventos := []record.Event{
		evento(t, 1, 0, record.AttemptStarted{Deployment: despliegueDePrueba(t), Runner: "img"}),
		evento(t, 2, 1, record.StepStarted{StepID: "01-test"}),
		evento(t, 3, 2, record.CommandStarted{StepID: "01-test", CommandName: "compila"}),
	}

	resultado := record.Fold(eventos)
	require.Equal(t, record.AttemptInterrupted, resultado.Status)

	assert.Empty(t, record.CheckStrip(eventos, resultado))
}

// UN COMANDO ABIERTO SÍ EXIGE RECORRER, porque el pliegue sólo los CUENTA: el
// detalle por comando vive en los hechos y duplicarlo en el resultado sería la
// divergencia que un modelo derivado existe para evitar.
func TestCheckStrip_UnComandoAbiertoSeDetectaRecorriendoLaTira(t *testing.T) {
	eventos := []record.Event{
		evento(t, 1, 0, record.AttemptStarted{Deployment: despliegueDePrueba(t), Runner: "img"}),
		evento(t, 2, 1, record.StepStarted{StepID: "01-test"}),
		evento(t, 3, 2, record.CommandStarted{StepID: "01-test", CommandName: "compila"}),
		evento(t, 4, 3, record.StepFinished{
			StepID: "01-test", Status: command.StepSuccess, Reason: command.ReasonNoRecord,
		}),
		evento(t, 5, 4, record.AttemptFinished{Status: record.AttemptSucceeded}),
	}

	resultado := record.Fold(eventos)
	require.Zero(t, resultado.Commands, "el pliegue cuenta cierres, y aquí no hubo ninguno")

	faults := record.CheckStrip(eventos, resultado)

	require.Len(t, faults, 1)
	assert.Equal(t, record.FaultParAbierto, faults[0].Kind)
	assert.Contains(t, faults[0].Detail, "01-test/compila")
}

// `sync_failed` DESPUÉS DEL DESENLACE NO ES UN DEFECTO, y hay que fijarlo porque
// la tentación de exigir que `attempt_finished` sea el último hecho es fuerte.
//
// No lo es por construcción: el empuje de cierre ocurre después del desenlace
// (spec 21 §9.3). Exigir el orden convertiría un empuje fallido —que es
// justamente lo que hay que poder diagnosticar— en una acusación de corrupción.
func TestCheckStrip_UnSyncFailedTrasElDesenlaceNoEsUnDefecto(t *testing.T) {
	eventos := append(tiraSana(t), evento(t, 7, 20, record.SyncFailed{
		Destination: "local:/mnt/vex-state", Cause: "no such file or directory",
	}))

	assert.Empty(t, record.CheckStrip(eventos, record.Fold(eventos)))
}

// ── El destino válido, que es lo que el listado tiene que marcar ────────────

func TestAttemptResult_DestinoValido(t *testing.T) {
	t.Run("un intento exitoso con todos sus steps cerrados lo es", func(t *testing.T) {
		eventos := tiraSana(t)

		assert.True(t, record.Fold(eventos).IsValidTarget())
	})

	t.Run("un step que revivió cuenta como correcto", func(t *testing.T) {
		eventos := []record.Event{
			evento(t, 1, 0, record.AttemptStarted{Deployment: despliegueDePrueba(t), Runner: "img"}),
			evento(t, 2, 1, record.StepStarted{StepID: "01-test"}),
			evento(t, 3, 2, record.StepFinished{
				StepID:    "01-test",
				Status:    command.StepCached,
				FromCache: true,
				Reason:    command.ReasonUpToDate,
			}),
			evento(t, 4, 3, record.AttemptFinished{Status: record.AttemptSucceeded}),
		}

		assert.True(t, record.Fold(eventos).IsValidTarget(),
			"no ejecutó nada, pero su resultado es el que estaba vigente")
	})

	t.Run("un intento interrumpido no lo es", func(t *testing.T) {
		eventos := tiraSana(t)[:3]

		assert.False(t, record.Fold(eventos).IsValidTarget())
	})

	t.Run("un intento exitoso con un step abierto tampoco", func(t *testing.T) {
		eventos := []record.Event{
			evento(t, 1, 0, record.AttemptStarted{Deployment: despliegueDePrueba(t), Runner: "img"}),
			evento(t, 2, 1, record.StepStarted{StepID: "01-test"}),
			evento(t, 3, 2, record.AttemptFinished{Status: record.AttemptSucceeded}),
		}

		assert.False(t, record.Fold(eventos).IsValidTarget(),
			"de un par abierto no se sabe si llegó a hacer lo que dice")
	})

	t.Run("un intento sin un solo step no lo es", func(t *testing.T) {
		eventos := []record.Event{
			evento(t, 1, 0, record.AttemptStarted{Deployment: despliegueDePrueba(t), Runner: "img"}),
			evento(t, 2, 1, record.AttemptFinished{Status: record.AttemptSucceeded}),
		}

		assert.False(t, record.Fold(eventos).IsValidTarget())
	})
}

// tiraSana son seis hechos consecutivos de un intento que terminó bien.
func tiraSana(t *testing.T) []record.Event {
	t.Helper()

	return []record.Event{
		evento(t, 1, 0, record.AttemptStarted{Deployment: despliegueDePrueba(t), Runner: "img"}),
		evento(t, 2, 1, record.StepStarted{StepID: "02-supply"}),
		evento(t, 3, 2, record.CommandStarted{StepID: "02-supply", CommandName: "provisiona"}),
		evento(t, 4, 3, record.CommandFinished{
			StepID:      "02-supply",
			CommandName: "provisiona",
			Status:      command.CommandSuccess,
			ExitCode:    0,
		}),
		evento(t, 5, 4, record.StepFinished{
			StepID: "02-supply", Status: command.StepSuccess, Reason: command.ReasonNoRecord,
		}),
		evento(t, 6, 5, record.AttemptFinished{Status: record.AttemptSucceeded}),
	}
}
