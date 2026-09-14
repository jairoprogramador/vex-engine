package deployment_test

// El ancla de un rollback (spec 28 §5.1', §5.2).
//
// Lo que se fija aquí es que el ancla **no puede apuntar a nada sin decirlo**:
// las tres formas de que un destino no sirva —no compone, no declara los mismos
// steps, no los declara en el mismo ámbito— fallan al componer o al validar, no
// se convierten en un rollback que ancla cero steps y termina bien.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/state"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/step"
)

func despliegueDePrueba(t *testing.T, par string) deployment.DeploymentID {
	t.Helper()
	id, err := deployment.ParseDeploymentID(
		deployment.DeploymentIDVersion + ":" + strings.Repeat(par, 32))
	require.NoError(t, err)
	return id
}

func destinoDePrueba(t *testing.T) deployment.RollbackTarget {
	t.Helper()
	target, err := deployment.NewRollbackTarget(
		despliegueDePrueba(t, "ab"), deployment.FirstAttempt())
	require.NoError(t, err)
	return target
}

func claveDePrueba(t *testing.T, stepID string) state.Key {
	t.Helper()
	key, err := state.NewKey(sujetoBase, state.NewProjectScope(), stepID)
	require.NoError(t, err)
	return key
}

func registroDePrueba(t *testing.T) state.RecordID {
	t.Helper()
	id, err := state.ParseRecordID("01J0000000000000000000000A")
	require.NoError(t, err)
	return id
}

// El par se compone con los value objects del paquete, así que un identificador
// mal escrito o un intento cero fallan EN EL BORDE en vez de convertirse en un
// ancla vacía —o sea, en un despliegue normal donde el usuario pidió volver
// atrás—.
func TestRollbackTarget(t *testing.T) {
	t.Run("el par se compone desde su forma externa", func(t *testing.T) {
		target, err := deployment.ParseRollbackTarget(
			despliegueDePrueba(t, "ab").String(), 1)
		require.NoError(t, err)

		assert.False(t, target.IsZero())
		assert.Equal(t, 1, target.Attempt().Number())
		assert.Contains(t, target.String(), "intento 1")
	})

	t.Run("un identificador sin forma es un error", func(t *testing.T) {
		_, err := deployment.ParseRollbackTarget("no-es-un-despliegue", 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "rollback_to")
	})

	t.Run("el intento 0 no es un intento", func(t *testing.T) {
		_, err := deployment.ParseRollbackTarget(despliegueDePrueba(t, "ab").String(), 0)
		require.Error(t, err)
	})

	t.Run("el valor cero es la ejecución normal", func(t *testing.T) {
		var target deployment.RollbackTarget
		assert.True(t, target.IsZero())
		assert.Empty(t, target.String())
	})
}

// Un ancla sin sujeto, sin contenido o sin ni un step no se puede componer: las
// tres producirían un rollback que no ancla nada y no lo dice, que es el modo de
// fallo silencioso que §4 rechaza en la alternativa D.
func TestNewRollbackAnchor_MaterialIncompletoEsError(t *testing.T) {
	contenido := materialBase(t).componer(t)
	pasos := []deployment.AnchoredStep{{StepID: "01-test", Scope: ""}}

	t.Run("sin destino", func(t *testing.T) {
		_, err := deployment.NewRollbackAnchor(
			deployment.RollbackTarget{}, contenido.ID(), pasos)
		require.Error(t, err)
	})

	t.Run("sin contenido", func(t *testing.T) {
		_, err := deployment.NewRollbackAnchor(
			destinoDePrueba(t), deployment.ContentID{}, pasos)
		require.Error(t, err)
	})

	t.Run("sin ni un step", func(t *testing.T) {
		_, err := deployment.NewRollbackAnchor(destinoDePrueba(t), contenido.ID(), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ni un step")
	})
}

// La consulta que la cadena de step hace: qué registro estuvo vigente.
//
// Las dos ausencias —el step no está, o está y no dejó registro— dan la misma
// respuesta, y es la correcta: aguas arriba «no hay registro» significa ejecutar.
func TestRollbackAnchor_AnchoredRecord(t *testing.T) {
	contenido := materialBase(t).componer(t)
	clave := claveDePrueba(t, "02-supply")
	registro := registroDePrueba(t)

	anchor, err := deployment.NewRollbackAnchor(destinoDePrueba(t), contenido.ID(),
		[]deployment.AnchoredStep{
			{StepID: "01-test", Scope: ""},
			{StepID: "02-supply", Scope: "project", Key: clave, RecordID: registro},
		})
	require.NoError(t, err)

	t.Run("el step que dejó registro", func(t *testing.T) {
		key, id, ok := anchor.AnchoredRecord("02-supply")
		require.True(t, ok)
		assert.True(t, key.Equals(clave))
		assert.Equal(t, registro.String(), id.String())
	})

	t.Run("el step que no dejó ninguno", func(t *testing.T) {
		_, _, ok := anchor.AnchoredRecord("01-test")
		assert.False(t, ok, "sin config.yaml, sin comandos o sin rules no hay a qué volver")
	})

	t.Run("un step que no estaba en el destino", func(t *testing.T) {
		_, _, ok := anchor.AnchoredRecord("09-inexistente")
		assert.False(t, ok)
	})

	t.Run("es inmutable: Steps devuelve una copia", func(t *testing.T) {
		pasos := anchor.Steps()
		pasos[0].StepID = "manipulado"

		otra := anchor.Steps()
		assert.Equal(t, "01-test", otra[0].StepID,
			"lo que R escribe no puede cambiar de qué registro parten sus steps")
	})
}

// La tercera condición de §5.2, que la spec 24 puso al descubierto: el destino
// tiene que declarar la MISMA lista de steps —y los mismos ámbitos— que la
// operación que se va a ejecutar.
func TestRollbackAnchor_Validate(t *testing.T) {
	contenido := materialBase(t).componer(t)

	// `01-test` no declara ámbito (no tiene `config.yaml`) y `02-supply` declara
	// `project`. Ver `stepTest` y `stepSupply`.
	coincidente := []deployment.AnchoredStep{
		{StepID: "01-test", Scope: ""},
		{StepID: "02-supply", Scope: "project"},
	}

	t.Run("la misma lista y los mismos ámbitos", func(t *testing.T) {
		anchor := anclaCon(t, contenido, coincidente)
		require.NoError(t, anchor.Validate(contenido))
	})

	t.Run("un step que la operación de hoy ya no declara", func(t *testing.T) {
		anchor := anclaCon(t, contenido, append(
			append([]deployment.AnchoredStep{}, coincidente...),
			deployment.AnchoredStep{StepID: "03-package", Scope: "project"}))

		err := anchor.Validate(contenido)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no declara los mismos steps")
		assert.Contains(t, err.Error(), "03-package", "nombra la que falta")
	})

	t.Run("un step renumerado deja de ser el mismo step", func(t *testing.T) {
		anchor := anclaCon(t, contenido, []deployment.AnchoredStep{
			{StepID: "01-test", Scope: ""},
			{StepID: "03-supply", Scope: "project"},
		})

		err := anchor.Validate(contenido)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "03-supply")
		assert.Contains(t, err.Error(), "02-supply")
	})

	// El caso que `01-test` demuestra sin renumerar nada (spec 24, recuadro):
	// conserva su `step_id` a través de un cambio de ámbito y aun así su ancla
	// apunta a una clave de estado que ya no se consulta.
	t.Run("el mismo nombre en otro ámbito no es el mismo sitio", func(t *testing.T) {
		anchor := anclaCon(t, contenido, []deployment.AnchoredStep{
			{StepID: "01-test", Scope: ""},
			{StepID: "02-supply", Scope: "environment"},
		})

		err := anchor.Validate(contenido)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "02-supply")
		assert.Contains(t, err.Error(), "clave de estado que ya no se consulta")
	})

	t.Run("el ancla cero no valida nada: es la ejecución normal", func(t *testing.T) {
		var anchor deployment.RollbackAnchor
		assert.True(t, anchor.IsZero())
		require.NoError(t, anchor.Validate(contenido))
	})
}

// `Matches` responde si la operación de hoy es la MISMA intención que la
// anclada, y su falso NO es un error: es «esto se parece a un rollback y no lo es
// del todo». Abortar contradiría a §7 —un step cuyo `commands.yaml` cambió entre
// E y R **se ejecuta**—.
func TestRollbackAnchor_Matches(t *testing.T) {
	contenido := materialBase(t).componer(t)

	otro := materialBase(t)
	otro.destination = "prod"
	distinto := otro.componer(t)

	anchor := anclaCon(t, contenido, []deployment.AnchoredStep{
		{StepID: "01-test", Scope: ""},
		{StepID: "02-supply", Scope: "project"},
	})

	assert.True(t, anchor.Matches(contenido))
	assert.False(t, anchor.Matches(distinto))

	var cero deployment.RollbackAnchor
	assert.False(t, cero.Matches(contenido), "el ancla cero no coincide con nada")
}

// El ancla satisface el puerto mínimo que la cadena de step declara. La
// comprobación vive en el cableado real; aquí se fija que el tipo lo cumple, que
// es lo que impide romperlo desde este lado sin enterarse.
func TestRollbackAnchor_SatisfaceElPuertoDeLaCadenaDeStep(t *testing.T) {
	var _ step.AnchorLookup = deployment.RollbackAnchor{}
}

func anclaCon(
	t *testing.T,
	contenido deployment.Content,
	pasos []deployment.AnchoredStep) deployment.RollbackAnchor {

	t.Helper()
	anchor, err := deployment.NewRollbackAnchor(destinoDePrueba(t), contenido.ID(), pasos)
	require.NoError(t, err)
	return anchor
}
