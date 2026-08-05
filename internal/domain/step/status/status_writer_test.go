package status_test

// El único escritor del estado de re-ejecución (spec 09 §5.2).
//
// Lo que se afirma aquí es el reparto: cada evidencia acaba en el repositorio de
// su regla y con la clave de su regla —las cuatro claves siguen siendo
// distintas, que es lo que la spec 10 unifica—, y una evidencia sin observación
// no se escribe en absoluto.

import (
	"testing"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/step/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writerDePrueba() (*status.StatusWriter, *fakeInstRepo, *fakeVarsRepo, *fakeCodeRepo, *fakeTimeRepo) {
	inst := &fakeInstRepo{}
	vars := &fakeVarsRepo{}
	code := &fakeCodeRepo{}
	tiempo := &fakeTimeRepo{}
	return status.NewStatusWriter(inst, vars, code, tiempo), inst, vars, code, tiempo
}

func TestStatusWriter_CadaEvidenciaVaASuAlmacenYConSuClave(t *testing.T) {
	writer, inst, vars, code, tiempo := writerDePrueba()
	ahora := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

	err := writer.Write(baseContext(t), []status.Evidence{
		status.NewEvidence(status.InstPipelineRuleName, "huella-inst", "", true),
		status.NewEvidence(status.VariablesRuleName, "huella-vars", "", true),
		status.NewEvidence(status.CodeProjectRuleName, "v1:huella-code", "", true),
		status.NewEvidence(status.TimeRuleName, status.FormatEvidenceTime(ahora), "", true),
	})

	require.NoError(t, err)

	assert.Equal(t, []string{joinKey(projectURL, pipelineURL, stepName)}, inst.sets)
	assert.Equal(t, "huella-inst", inst.stored)

	assert.Equal(t, []string{joinKey(projectURL, pipelineURL, environment, stepName)}, vars.sets)
	assert.Equal(t, "huella-vars", vars.stored)

	assert.Equal(t, []string{joinKey(projectURL, pipelineURL, stepName)}, code.sets)
	assert.Equal(t, "v1:huella-code", code.stored,
		"el prefijo de versión de la spec 08 llega intacto al almacén")

	assert.Equal(t, []string{joinKey(projectURL, environment, stepName)}, tiempo.sets,
		"la clave del tiempo sigue sin pipeline")
	assert.True(t, ahora.Equal(tiempo.stored), "el instante se persiste sin perder precisión")
}

// Spec 09 §5.4, y FALLABA ANTES: `time_rule` solo refrescaba su marca cuando
// ELLA decidía ejecutar. Un step re-ejecutado porque cambió el código no
// reiniciaba su TTL, así que el TTL medía «cuándo expiró por última vez» en vez
// de «cuándo se ejecutó por última vez» — algo que nadie quiso medir.
//
// Ahora la evidencia del tiempo lleva el instante actual aunque la regla diga
// `Skip`, y el escritor la persiste porque el step se ejecutó. Este test junta
// las dos piezas: la policy que decide y el escritor que persiste.
func TestStatusWriter_ElTTLSeRefrescaAunqueQuienMandeEjecutarSeaOtraRegla(t *testing.T) {
	ahora := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	haceUnaSemana := ahora.Add(-7 * 24 * time.Hour)

	repoTiempo := &fakeTimeRepo{stored: haceUnaSemana}
	repoCodigo := &fakeCodeRepo{stored: "v1:huella-vieja"}

	policy := status.NewPolicy("test",
		status.NewCodeProjectRuleRule(repoCodigo), // dice Run: el código cambió
		status.NewTimeRule(repoTiempo),            // dice Skip: quedan 23 días de TTL
	)

	ctx := baseContext(t)
	ctx[status.ProjectStatusCurrentParam] = "v1:huella-nueva"
	ctx[status.CurrentTimeParam] = ahora

	decision, evidencias, err := policy.Evaluate(ctx)
	require.NoError(t, err)
	require.True(t, decision.ShouldRun())
	require.False(t, decision.IsUndetermined(), "se ejecuta por el cambio de código")

	// El step corre y termina bien: aquí es donde se escribe.
	writer := status.NewStatusWriter(&fakeInstRepo{}, &fakeVarsRepo{}, repoCodigo, repoTiempo)
	require.NoError(t, writer.Write(ctx, evidencias))

	assert.True(t, ahora.Equal(repoTiempo.stored),
		"el TTL se reinicia cuando el step se ejecuta, no cuando la regla del tiempo decide")
	assert.Equal(t, "v1:huella-nueva", repoCodigo.stored)
}

func TestStatusWriter_UnaEvidenciaSinObservacionNoSeEscribe(t *testing.T) {
	// Escribir el vacío sería peor que no escribir: la corrida siguiente
	// encontraría una huella vacía que coincide con otra huella vacía y saltaría
	// el step por una igualdad que nadie observó.
	writer, inst, _, _, _ := writerDePrueba()

	err := writer.Write(baseContext(t), []status.Evidence{
		status.NoEvidence(status.InstPipelineRuleName),
	})

	require.NoError(t, err)
	assert.Empty(t, inst.sets)
}

func TestStatusWriter_ElFalloDeUnaEscrituraNoImpideLasDemas(t *testing.T) {
	inst := &fakeInstRepo{setErr: errRepositorioCaido}
	vars := &fakeVarsRepo{}
	code := &fakeCodeRepo{}
	tiempo := &fakeTimeRepo{}
	writer := status.NewStatusWriter(inst, vars, code, tiempo)

	err := writer.Write(baseContext(t), []status.Evidence{
		status.NewEvidence(status.InstPipelineRuleName, "huella-inst", "", true),
		status.NewEvidence(status.CodeProjectRuleName, "v1:huella-code", "", true),
	})

	require.ErrorIs(t, err, errRepositorioCaido)
	assert.Contains(t, err.Error(), status.InstPipelineRuleName,
		"el error nombra la entrada que se perdió")
	assert.Equal(t, "v1:huella-code", code.stored,
		"perder una entrada no es motivo para perder las otras tres")
}

func TestStatusWriter_UnaReglaDesconocidaEsUnError(t *testing.T) {
	writer, _, _, _, _ := writerDePrueba()

	err := writer.Write(baseContext(t), []status.Evidence{
		status.NewEvidence("regla_inventada", "algo", "", true),
	})

	require.Error(t, err, "una observación sin almacén se reporta, no se pierde en silencio")
	assert.Contains(t, err.Error(), "regla_inventada")
}

func TestStatusWriter_SinClaveNoEscribe(t *testing.T) {
	// La clave sale del RuleContext. Si falta, no hay dónde escribir, y eso es un
	// error visible en vez de una entrada guardada bajo una clave a medias.
	writer, inst, _, _, _ := writerDePrueba()

	err := writer.Write(status.RuleContext{}, []status.Evidence{
		status.NewEvidence(status.InstPipelineRuleName, "huella-inst", "", true),
	})

	require.Error(t, err)
	assert.Empty(t, inst.sets)
}
