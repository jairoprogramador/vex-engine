package step_test

// Las dos políticas de «qué registro está vigente» (spec 28 §5.2', §5.3').
//
// Lo que se fija aquí es la SUSTITUIBILIDAD: las dos implementaciones tienen la
// misma firma y la misma asimetría, así que el bucle de decisión no puede notar
// cuál está puesta. Es la comprobación de que un rollback no relaja nada — si
// necesitara un trato especial, estaría saltándose algo.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domState "github.com/jairoprogramador/vex-engine/old-internal/domain/state"
	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
)

// almacenDeDosRegistros tiene un «último» y uno CONCRETO distinto de él, que es
// la situación que un rollback existe para distinguir.
type almacenDeDosRegistros struct {
	ultimo   domState.StepRecord
	porID    map[string]domState.StepRecord
	pedidos  []string
	fallaGet error
}

var _ domState.Records = (*almacenDeDosRegistros)(nil)

func (a *almacenDeDosRegistros) Last(
	*context.Context, domState.Key) (domState.StepRecord, bool, error) {
	a.pedidos = append(a.pedidos, "last")
	return a.ultimo, !a.ultimo.ID().IsZero(), nil
}

func (a *almacenDeDosRegistros) Get(
	_ *context.Context, _ domState.Key, id domState.RecordID) (domState.StepRecord, bool, error) {

	a.pedidos = append(a.pedidos, "get:"+id.String())
	if a.fallaGet != nil {
		return domState.StepRecord{}, false, a.fallaGet
	}
	record, found := a.porID[id.String()]
	return record, found, nil
}

func (a *almacenDeDosRegistros) Append(
	*context.Context, domState.Key, domState.StepRecord) error {
	return nil
}

// anclaFalsa es lo mínimo que el proveedor anclado necesita saber, que es
// exactamente el puerto que `step` declara: `deployment.RollbackAnchor` lo
// satisface desde el otro lado sin que este paquete lo nombre.
type anclaFalsa struct {
	stepID   string
	key      domState.Key
	recordID domState.RecordID
}

var _ domStep.AnchorLookup = anclaFalsa{}

func (a anclaFalsa) AnchoredRecord(stepID string) (domState.Key, domState.RecordID, bool) {
	if stepID != a.stepID {
		return domState.Key{}, domState.RecordID{}, false
	}
	return a.key, a.recordID, true
}

func registroConID(t *testing.T, ulid, fingerprint string) domState.StepRecord {
	t.Helper()
	id, err := domState.ParseRecordID(ulid)
	require.NoError(t, err)
	record, err := domState.NewStepRecord(id, fingerprint, nil,
		domState.Provenance{ExecutionID: "exec-" + ulid, At: time.Unix(0, 1).UTC()})
	require.NoError(t, err)
	return record
}

func claveDe(t *testing.T, scope domState.Scope, stepID string) domState.Key {
	t.Helper()
	key, err := domState.NewKey("https://vex.test/acme/demo-app", scope, stepID)
	require.NoError(t, err)
	return key
}

const (
	ulidViejo = "01J0000000000000000000000A"
	ulidNuevo = "01J0000000000000000000000B"
)

func almacenDePrueba(t *testing.T) *almacenDeDosRegistros {
	t.Helper()
	viejo := registroConID(t, ulidViejo, "sf-v1:viejo")
	nuevo := registroConID(t, ulidNuevo, "sf-v1:nuevo")
	return &almacenDeDosRegistros{
		ultimo: nuevo,
		porID: map[string]domState.StepRecord{
			ulidViejo: viejo,
			ulidNuevo: nuevo,
		},
	}
}

func TestLastRecordProvider_DevuelveElUltimo(t *testing.T) {
	ctx := context.Background()
	almacen := almacenDePrueba(t)

	record, found, err := domStep.NewLastRecordProvider(almacen).
		Current(&ctx, claveDe(t, domState.NewProjectScope(), "02-supply"))

	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, ulidNuevo, record.ID().String())
	assert.Equal(t, []string{"last"}, almacen.pedidos,
		"la política normal no pregunta por ningún registro concreto")
}

func TestAnchoredRecordProvider(t *testing.T) {
	clave := claveDe(t, domState.NewProjectScope(), "02-supply")
	viejo, err := domState.ParseRecordID(ulidViejo)
	require.NoError(t, err)

	t.Run("devuelve el ANCLADO, no el último", func(t *testing.T) {
		ctx := context.Background()
		almacen := almacenDePrueba(t)

		record, found, err := domStep.NewAnchoredRecordProvider(almacen,
			anclaFalsa{stepID: "02-supply", key: clave, recordID: viejo}).Current(&ctx, clave)

		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, ulidViejo, record.ID().String())
		assert.Equal(t, []string{"get:" + ulidViejo}, almacen.pedidos,
			"el ancla no consulta el último ni para desempatar")
	})

	// La carga lee los DOS ámbitos y sólo uno es el declarado (spec 13 §5.4). El
	// otro no tuvo registro vigente en el intento anclado, y devolver el del
	// ámbito equivocado sería inventarse un ancla.
	t.Run("otra clave del mismo step es una ausencia", func(t *testing.T) {
		ctx := context.Background()
		almacen := almacenDePrueba(t)
		otroAmbito, err := domState.NewEnvironmentScope("sand")
		require.NoError(t, err)

		_, found, err := domStep.NewAnchoredRecordProvider(almacen,
			anclaFalsa{stepID: "02-supply", key: clave, recordID: viejo}).
			Current(&ctx, claveDe(t, otroAmbito, "02-supply"))

		require.NoError(t, err)
		assert.False(t, found)
		assert.Empty(t, almacen.pedidos, "no llega a preguntarle nada al almacén")
	})

	// Un step que no dejó registro en el intento anclado —sin `config.yaml`, sin
	// comandos o sin `rules`— se comporta como un step sin registro: se ejecuta.
	t.Run("un step sin ancla es una ausencia y NO un error", func(t *testing.T) {
		ctx := context.Background()
		almacen := almacenDePrueba(t)

		_, found, err := domStep.NewAnchoredRecordProvider(almacen,
			anclaFalsa{stepID: "otro", key: clave, recordID: viejo}).Current(&ctx, clave)

		require.NoError(t, err)
		assert.False(t, found)
	})

	// La otra mitad de la asimetría, y es la que no se puede relajar: un registro
	// ILEGIBLE es un error, porque esa duda no se resuelve ejecutando sin
	// arriesgar un recurso duplicado (spec 11 §5.6).
	t.Run("un registro ilegible sigue siendo un error", func(t *testing.T) {
		ctx := context.Background()
		almacen := almacenDePrueba(t)
		almacen.fallaGet = assert.AnError

		_, _, err := domStep.NewAnchoredRecordProvider(almacen,
			anclaFalsa{stepID: "02-supply", key: clave, recordID: viejo}).Current(&ctx, clave)

		require.Error(t, err)
	})
}

// La indirección que hace que los tres handlers dependan del PUERTO: arranca en
// «el último» —la ejecución normal— y el handler 09 instala el ancla antes del
// primer step.
func TestCurrentRecords_LaPoliticaSeEligeUnaVez(t *testing.T) {
	ctx := context.Background()
	clave := claveDe(t, domState.NewProjectScope(), "02-supply")
	viejo, err := domState.ParseRecordID(ulidViejo)
	require.NoError(t, err)

	almacen := almacenDePrueba(t)
	current := domStep.NewCurrentRecords(almacen)

	record, found, err := current.Current(&ctx, clave)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, ulidNuevo, record.ID().String(), "sin ancla, el último")

	current.UseAnchor(anclaFalsa{stepID: "02-supply", key: clave, recordID: viejo})

	record, found, err = current.Current(&ctx, clave)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, ulidViejo, record.ID().String(), "con ancla, el anclado")
}
