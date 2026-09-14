package state_test

// El registro: agregado inmutable, y la única pregunta que decide algo
// (spec 11 §5.3, §5.1').

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/state"
)

var instante = time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

func TestStepRecord_RevivirExigeIgualdadDeHuella(t *testing.T) {
	registro := registroDePrueba(t, "ck-v1:aaa", nil)

	assert.True(t, registro.Revives("ck-v1:aaa"))
	assert.False(t, registro.Revives("ck-v1:bbb"))
}

// EL test que fija el argumento con el que D-A14 se cerró al revés (spec 11
// §5.2): dos pipelines comparten clave de estado y NO se reviven entre sí,
// porque sus huellas difieren por construcción —la huella incluye el
// pipelinecode entero—.
func TestStepRecord_DosPipelinesNoSeRevivenEntreSi(t *testing.T) {
	delPrimerPipeline := registroDePrueba(t, "ck-v1:huella-del-pipeline-a", nil)

	assert.False(t, delPrimerPipeline.Revives("ck-v1:huella-del-pipeline-b"),
		"compartir clave no es compartir estado: lo que revive es la huella")
}

// Una huella vacía nunca revive, de ninguno de los dos lados. Es lo que conserva
// «sin evidencia ⇒ ejecutar» (spec 05 §5.1) por construcción, y lo que hace
// seguro guardar el registro de un step cuyo material no se pudo componer: el
// ARN se conserva, el salto no se concede.
func TestStepRecord_UnaHuellaVaciaNuncaRevive(t *testing.T) {
	sinHuella := registroDePrueba(t, "", nil)
	conHuella := registroDePrueba(t, "ck-v1:aaa", nil)

	assert.False(t, sinHuella.Revives("ck-v1:aaa"))
	assert.False(t, sinHuella.Revives(""))
	assert.False(t, conHuella.Revives(""))
}

// Inmutable de verdad: el registro no comparte el slice con quien lo construyó
// ni con quien lo lee, así que nadie puede modificarlo por debajo. La
// inmutabilidad ES la implementación del append-only.
func TestStepRecord_EsInmutable(t *testing.T) {
	original := []command.Variable{variable(t, "acr_name", "acme.azurecr.io")}
	registro := registroDePrueba(t, "ck-v1:aaa", original)

	original[0] = variable(t, "acr_name", "otro.azurecr.io")
	leidas := registro.Variables()
	require.Len(t, leidas, 1)
	assert.Equal(t, "acme.azurecr.io", leidas[0].Value())

	leidas[0] = variable(t, "acr_name", "tercero.azurecr.io")
	assert.Equal(t, "acme.azurecr.io", registro.Variables()[0].Value())
}

func TestStepRecord_LaProcedenciaEsObligatoria(t *testing.T) {
	id := idDePrueba(t, instante)

	casos := []struct {
		nombre     string
		id         state.RecordID
		producedBy state.Provenance
	}{
		{"sin record_id", state.RecordID{}, state.Provenance{ExecutionID: "exec-1", At: instante}},
		{"sin ejecución", id, state.Provenance{At: instante}},
		{"sin instante", id, state.Provenance{ExecutionID: "exec-1"}},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			_, err := state.NewStepRecord(caso.id, "ck-v1:aaa", nil, caso.producedBy)
			assert.Error(t, err)
		})
	}
}

// El registro sin atribuir es lo que devuelve un almacén que guarda el último
// valor y no historia (el adaptador de Supabase). Sus consecuencias son
// exactamente las correctas: conserva las variables y no concede nada.
func TestStepRecord_SinAtribuirConservaLasVariablesYNoRevive(t *testing.T) {
	registro := state.NewUnattributedRecord(
		[]command.Variable{variable(t, "acr_name", "acme.azurecr.io")})

	require.Len(t, registro.Variables(), 1)
	assert.True(t, registro.ID().IsZero(), "no puede entrar en el índice")
	assert.False(t, registro.Revives("ck-v1:aaa"))
	assert.False(t, registro.Revives(""))
}

func registroDePrueba(t *testing.T, huella string, variables []command.Variable) state.StepRecord {
	t.Helper()
	registro, err := state.NewStepRecord(
		idDePrueba(t, instante), huella, variables,
		state.Provenance{ExecutionID: "exec-1", At: instante})
	require.NoError(t, err)
	return registro
}

func idDePrueba(t *testing.T, at time.Time) state.RecordID {
	t.Helper()
	id, err := state.NewRecordID(at, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	require.NoError(t, err)
	return id
}

func variable(t *testing.T, nombre, valor string) command.Variable {
	t.Helper()
	v, err := command.NewVariable(nombre, valor, command.OriginRuntime)
	require.NoError(t, err)
	return v
}
