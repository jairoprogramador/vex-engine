package record_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/state"
)

// origen es el instante base de los hechos de estos tests. Fijo y explícito:
// aquí no hay reloj, y esa es media tesis del paquete.
var origen = time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)

// evento compone un hecho con una posición y un desplazamiento en segundos sobre
// el instante base. El `event_id` se deriva del instante, así que dos hechos del
// mismo segundo con la misma entropía colisionarían: el desplazamiento los separa.
func evento(t *testing.T, position uint64, segundos int, payload record.Payload) record.Event {
	t.Helper()

	instante := origen.Add(time.Duration(segundos) * time.Second)
	id, err := record.NewEventID(instante, entropiaFija)
	require.NoError(t, err)
	seq, err := record.NewSeq(position)
	require.NoError(t, err)

	hecho, err := record.NewEvent(id, seq, instante, deployment.FirstAttempt(), payload)
	require.NoError(t, err)
	return hecho
}

// despliegueDePrueba es un `deployment_id` fijo, para que un `attempt_started`
// se pueda componer sin arrastrar todo el material de un `Content`.
func despliegueDePrueba(t *testing.T) deployment.DeploymentID {
	t.Helper()

	id, err := deployment.ParseDeploymentID(
		deployment.DeploymentIDVersion + ":" + strings.Repeat("ab", 32))
	require.NoError(t, err)
	return id
}

func ambienteDePrueba(t *testing.T) state.Scope {
	t.Helper()

	scope, err := state.NewEnvironmentScope("sand")
	require.NoError(t, err)
	return scope
}

// evidenciaDePrueba es una referencia COMPLETA a un registro: la que un
// `step_finished` lleva tanto si el step revivió como si ejecutó.
func evidenciaDePrueba(t *testing.T) record.EvidenceRef {
	t.Helper()

	key, err := state.NewKey("https://vex.test/acme/demo-app", ambienteDePrueba(t), "02-supply")
	require.NoError(t, err)
	recordID, err := state.NewRecordID(origen.Add(-24*time.Hour), []byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a})
	require.NoError(t, err)

	return record.EvidenceRef{
		ExecutionID: "e5a1f0c2-0000-4000-8000-000000000001",
		At:          origen.Add(-24 * time.Hour),
		StateKey:    key,
		RecordID:    recordID,
	}
}

func codigo(valor int) *int { return &valor }
