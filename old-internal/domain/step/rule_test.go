package step_test

// Las reglas de re-ejecución y su combinador (spec 15).
//
// Lo que se prueba aquí es el VOCABULARIO y el ÁLGEBRA: qué se puede declarar,
// qué se rechaza, y qué sale de combinar lo declarado. La conexión con el resto
// del motor —que la huella se componga con el alcance declarado, que un step sin
// reglas no persista— vive en `cache_material_test.go`, en
// `step_executable_test.go` y en el harness.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domState "github.com/jairoprogramador/vex-engine/old-internal/domain/state"
	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
)

const (
	huellaA = "ck-v1:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	huellaB = "ck-v1:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

var ahora = time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

// registroDe compone el último registro de una clave: la huella con la que se
// escribió y cuándo.
func registroDe(t *testing.T, fingerprint string, at time.Time) domState.StepRecord {
	t.Helper()
	id, err := domState.NewRecordID(at, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	require.NoError(t, err)
	record, err := domState.NewStepRecord(id, fingerprint, nil,
		domState.Provenance{ExecutionID: "exec-1", At: at})
	require.NoError(t, err)
	return record
}

func sujeto(t *testing.T, fingerprint string, edad time.Duration) domStep.RuleSubject {
	t.Helper()
	return domStep.RuleSubject{
		Fingerprint: fingerprint,
		Last:        registroDe(t, huellaA, ahora.Add(-edad)),
		Now:         ahora,
	}
}

// ── state_changed ───────────────────────────────────────────────────────────

// EL PAR QUE DA NOMBRE A LA GRANULARIDAD DECLARADA, visto desde el vocabulario:
// las fuentes no cambian lo que la regla EVALÚA, cambian lo que entra en la
// huella que se le da. Aquí se fija esa mitad; la otra —que la huella salga
// distinta— está en `cache_material_test.go`.
func TestStateChangedRule_LasFuentesDecidenQueEntraEnLaHuella(t *testing.T) {
	t.Run("la forma corta incluye el proyecto", func(t *testing.T) {
		// Es el default seguro del catálogo, y la dirección importa: si
		// `- state_changed` significara «sólo mi declaración», un `01-test` escrito
		// así dejaría de re-ejecutarse ante un cambio de código, que es la peor
		// omisión que este motor puede cometer.
		assert.True(t, domStep.NewDefaultStateChangedRule().WatchesProject())
	})

	t.Run("la forma larga completa produce lo mismo que la corta", func(t *testing.T) {
		regla, err := domStep.NewStateChangedRule([]string{"pipeline", "project"})
		require.NoError(t, err)
		assert.Equal(t, domStep.NewDefaultStateChangedRule(), regla,
			"'- state_changed' y '- state_changed: [pipeline, project]' son la misma declaración")
	})

	t.Run("sólo pipeline deja el código del proyecto fuera", func(t *testing.T) {
		regla, err := domStep.NewStateChangedRule([]string{"pipeline"})
		require.NoError(t, err)
		assert.False(t, regla.WatchesProject())
	})

	t.Run("el orden de las fuentes no significa nada", func(t *testing.T) {
		unOrden, err := domStep.NewStateChangedRule([]string{"pipeline", "project"})
		require.NoError(t, err)
		elOtro, err := domStep.NewStateChangedRule([]string{"project", "pipeline"})
		require.NoError(t, err)
		assert.Equal(t, unOrden, elOtro)
	})
}

func TestStateChangedRule_LoQueNoSePuedeDeclarar(t *testing.T) {
	casos := []struct {
		nombre    string
		fuentes   []string
		enElError string
	}{
		{
			nombre:    "pipeline no se puede omitir",
			fuentes:   []string{"project"},
			enElError: "no puede omitir 'pipeline'",
		},
		{
			nombre:    "una fuente inventada",
			fuentes:   []string{"pipeline", "templates"},
			enElError: "templates",
		},
		{
			nombre:    "la lista vacía no es la forma corta",
			fuentes:   []string{},
			enElError: "no vigila nada",
		},
		{
			nombre:    "una fuente repetida",
			fuentes:   []string{"pipeline", "pipeline"},
			enElError: "dos veces",
		},
		{
			nombre:    "el vocabulario es sensible a mayúsculas",
			fuentes:   []string{"Pipeline"},
			enElError: "Pipeline",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			_, err := domStep.NewStateChangedRule(caso.fuentes)

			require.Error(t, err)
			assert.Contains(t, err.Error(), caso.enElError)
		})
	}
}

// La guarda de la huella vacía viaja CON la regla, y tiene que viajar con ella:
// es lo que conserva «sin evidencia ⇒ ejecutar» (spec 05 §5.1) al mover la
// comparación fuera del handler.
func TestStateChangedRule_UnaHuellaVaciaNuncaRevive(t *testing.T) {
	regla := domStep.NewDefaultStateChangedRule()

	t.Run("la huella actual vacía", func(t *testing.T) {
		assert.True(t, regla.IsSatisfiedBy(sujeto(t, "", time.Hour)),
			"no se pudo componer la huella: no hay nada que comparar, se ejecuta")
	})

	t.Run("la huella del registro vacía", func(t *testing.T) {
		subject := domStep.RuleSubject{
			Fingerprint: huellaA,
			Last:        registroDe(t, "", ahora.Add(-time.Hour)),
			Now:         ahora,
		}
		assert.True(t, regla.IsSatisfiedBy(subject),
			"un registro que no dice de qué contenido es no puede afirmar que esté al día")
	})

	t.Run("las dos iguales revive", func(t *testing.T) {
		assert.False(t, regla.IsSatisfiedBy(sujeto(t, huellaA, time.Hour)))
	})

	t.Run("distintas se ejecuta", func(t *testing.T) {
		assert.True(t, regla.IsSatisfiedBy(sujeto(t, huellaB, time.Hour)))
	})
}

// ── max_age ─────────────────────────────────────────────────────────────────

func TestMaxAgeRule_LaGramaticaDeLaDuracion(t *testing.T) {
	t.Run("la sintaxis de time.ParseDuration", func(t *testing.T) {
		for _, texto := range []string{"30m", "6h", "24h", "720h", "1h30m"} {
			regla, err := domStep.NewMaxAgeRule(texto)
			require.NoError(t, err, texto)
			assert.Positive(t, regla.MaxAge())
		}
	})

	// Una sola gramática de duración en el binario: se conserva la del lenguaje
	// en vez de inventar una. `30d` es lo primero que alguien escribe, así que el
	// mensaje lo dice.
	t.Run("no hay unidad de días", func(t *testing.T) {
		_, err := domStep.NewMaxAgeRule("30d")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "720h")
		assert.Contains(t, err.Error(), "no hay unidad de días")
	})

	t.Run("una duración no positiva es un step sin reglas escrito raro", func(t *testing.T) {
		for _, texto := range []string{"0s", "-1h"} {
			_, err := domStep.NewMaxAgeRule(texto)
			assert.Error(t, err, texto)
		}
	})

	t.Run("lo que no es una duración", func(t *testing.T) {
		for _, texto := range []string{"", "siempre", "24 h"} {
			_, err := domStep.NewMaxAgeRule(texto)
			assert.Error(t, err, texto)
		}
	})
}

// El borde es EXCLUSIVO: el registro caduca CUANDO se alcanza su instante, no
// después. Es la misma frontera que comparaba el TTL global, conservada al
// cambiarle el dueño.
func TestMaxAgeRule_ElBordeEsExclusivo(t *testing.T) {
	regla, err := domStep.NewMaxAgeRule("24h")
	require.NoError(t, err)

	assert.False(t, regla.IsSatisfiedBy(sujeto(t, huellaA, 23*time.Hour+59*time.Minute)),
		"dentro de la ventana: revive")
	assert.True(t, regla.IsSatisfiedBy(sujeto(t, huellaA, 24*time.Hour)),
		"al alcanzar el instante ya caducó")
	assert.True(t, regla.IsSatisfiedBy(sujeto(t, huellaA, 400*24*time.Hour)))
}

// La expiración NO mira la huella, y la invalidación NO mira el tiempo. Son dos
// categorías distintas y por eso pueden pedirse por separado: mezclarlas fue lo
// que produjo el TTL global.
func TestLasDosReglasNoSePisan(t *testing.T) {
	maxAge, err := domStep.NewMaxAgeRule("24h")
	require.NoError(t, err)

	assert.False(t, maxAge.IsSatisfiedBy(sujeto(t, huellaB, time.Hour)),
		"la huella cambió y a max_age no le consta: por eso §5.4 avisa")
	assert.True(t,
		domStep.NewDefaultStateChangedRule().IsSatisfiedBy(sujeto(t, huellaB, 400*24*time.Hour)),
		"y state_changed decide igual con un registro de hace 400 días")

	assert.Equal(t, domStep.CategoryExpiration, maxAge.Category())
	assert.Equal(t, domStep.CategoryInvalidation, domStep.NewDefaultStateChangedRule().Category())
}

// ── El conjunto ─────────────────────────────────────────────────────────────

// LA INVARIANTE QUE ESTE TIPO CUSTODIA, y el sitio exacto donde es fácil
// perderla: el OR de un conjunto vacío es falso, así que la lectura literal
// diría «revivir». Eso es el defecto que le da título a la spec 05, reaparecido
// por una vía nueva.
func TestRuleSet_UnConjuntoVacioNoRevive(t *testing.T) {
	vacio := domStep.EmptyRuleSet()

	require.True(t, vacio.IsEmpty())

	kind, run := vacio.RequiresRun(sujeto(t, huellaA, time.Hour))

	assert.True(t, run,
		"«no hay nada que comprobar» no es «esto está al día»: un conjunto vacío de evidencia no concluye nada")
	assert.Equal(t, domStep.RuleKindNone, kind,
		"y no hay regla que nombrar: no se re-ejecuta POR una regla, sino por su ausencia")
}

// LA PRUEBA DE QUE ESTO ES UNA SPECIFICATION Y NO UNA `Policy` DISFRAZADA: un
// conjunto con una sola regla se comporta exactamente como esa regla.
func TestRuleSet_ConUnaSolaReglaEsEsaRegla(t *testing.T) {
	regla := domStep.NewDefaultStateChangedRule()
	set, err := domStep.NewRuleSet(regla)
	require.NoError(t, err)

	for _, caso := range []struct {
		nombre      string
		fingerprint string
	}{{"igual", huellaA}, {"distinta", huellaB}, {"vacía", ""}} {
		t.Run(caso.nombre, func(t *testing.T) {
			subject := sujeto(t, caso.fingerprint, 400*24*time.Hour)

			_, run := set.RequiresRun(subject)

			assert.Equal(t, regla.IsSatisfiedBy(subject), run)
		})
	}
}

// EL OR, en sus cuatro cuadrantes. Cada regla es una razón INDEPENDIENTE para
// desconfiar de lo guardado: exigir que coincidieran dos equivaldría a ignorar
// la primera que apareciera.
func TestRuleSet_LasReglasSeCombinanConOr(t *testing.T) {
	maxAge, err := domStep.NewMaxAgeRule("24h")
	require.NoError(t, err)
	set, err := domStep.NewRuleSet(domStep.NewDefaultStateChangedRule(), maxAge)
	require.NoError(t, err)

	casos := []struct {
		nombre      string
		fingerprint string
		edad        time.Duration
		run         bool
		kind        domStep.RuleKind
	}{
		{
			nombre: "huella distinta dentro de la ventana", fingerprint: huellaB,
			edad: time.Hour, run: true, kind: domStep.RuleKindStateChanged,
		},
		{
			nombre: "misma huella fuera de la ventana", fingerprint: huellaA,
			edad: 48 * time.Hour, run: true, kind: domStep.RuleKindMaxAge,
		},
		{
			nombre: "las dos razones a la vez: gana la primera declarada", fingerprint: huellaB,
			edad: 48 * time.Hour, run: true, kind: domStep.RuleKindStateChanged,
		},
		{
			nombre: "ninguna razón: revive", fingerprint: huellaA,
			edad: time.Hour, run: false, kind: domStep.RuleKindNone,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			kind, run := set.RequiresRun(sujeto(t, caso.fingerprint, caso.edad))

			assert.Equal(t, caso.run, run)
			assert.Equal(t, caso.kind, kind,
				"el motivo emitido es el de la PRIMERA regla que dijo que sí")
		})
	}
}

// Dos reglas de la misma clave no componen nada: con un OR, la primera gana
// siempre y la segunda es una contradicción que el motor no puede resolver.
func TestRuleSet_UnaReglaDeclaradaDosVecesEsUnError(t *testing.T) {
	unaHora, err := domStep.NewMaxAgeRule("1h")
	require.NoError(t, err)
	otra, err := domStep.NewMaxAgeRule("720h")
	require.NoError(t, err)

	_, err = domStep.NewRuleSet(unaHora, otra)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "max_age")
}

// `StateChanged` es lo que consulta el compositor del material —no el
// combinador—, y responde una pregunta distinta: no «¿se cumple?» sino «¿qué
// entra en la huella?».
func TestRuleSet_ElAlcanceDeLaHuellaSaleDeLaReglaDeclarada(t *testing.T) {
	t.Run("sin state_changed no hay huella que componer", func(t *testing.T) {
		maxAge, err := domStep.NewMaxAgeRule("24h")
		require.NoError(t, err)
		set, err := domStep.NewRuleSet(maxAge)
		require.NoError(t, err)

		_, declarada := set.StateChanged()

		assert.False(t, declarada,
			"calcular una huella que ninguna regla mira sería trabajo que nadie lee")
	})

	t.Run("con state_changed viaja su alcance", func(t *testing.T) {
		soloPipeline, err := domStep.NewStateChangedRule([]string{"pipeline"})
		require.NoError(t, err)
		set, err := domStep.NewRuleSet(soloPipeline)
		require.NoError(t, err)

		regla, declarada := set.StateChanged()

		require.True(t, declarada)
		assert.False(t, regla.WatchesProject())
	})
}

// La configuración válida y PELIGROSA de §5.4: expiración sin invalidación. No
// es un error y no se prohíbe —nada es implícito, todo se declara— pero el motor
// avisa, para que sea una elección consciente y no un olvido.
//
// Se pregunta por CATEGORÍA y no por `max_age`: la tercera regla de expiración
// que alguien añada entra aquí sola.
func TestRuleSet_ExpirarSinInvalidarSeAvisa(t *testing.T) {
	maxAge, err := domStep.NewMaxAgeRule("24h")
	require.NoError(t, err)

	soloExpira, err := domStep.NewRuleSet(maxAge)
	require.NoError(t, err)
	assert.True(t, soloExpira.ExpiresWithoutInvalidating())

	conInvalidacion, err := domStep.NewRuleSet(maxAge, domStep.NewDefaultStateChangedRule())
	require.NoError(t, err)
	assert.False(t, conInvalidacion.ExpiresWithoutInvalidating())

	assert.False(t, domStep.EmptyRuleSet().ExpiresWithoutInvalidating(),
		"sin reglas no hay nada de lo que avisar: el step se ejecuta siempre")
}

// El ámbito y las reglas van juntos en el archivo y son ortogonales: el ámbito
// es DÓNDE vive el estado, la regla es CUÁNDO deja de ser válido. Para dejar
// registro hacen falta los dos.
func TestStepConfig_ParaRecordarseHacenFaltaLasDosCosas(t *testing.T) {
	conReglas, err := domStep.NewRuleSet(domStep.NewDefaultStateChangedRule())
	require.NoError(t, err)

	declarado, err := domStep.NewStepConfig(domStep.NewProjectScope(), conReglas)
	require.NoError(t, err)
	assert.True(t, declarado.Remembers())

	sinReglas, err := domStep.NewStepConfig(domStep.NewProjectScope(), domStep.EmptyRuleSet())
	require.NoError(t, err)
	assert.True(t, sinReglas.IsDeclared(), "declara ámbito")
	assert.False(t, sinReglas.Remembers(), "y no tiene ninguna afirmación que guardar")

	assert.False(t, domStep.NoStepConfig().Remembers())
	assert.True(t, domStep.NoStepConfig().Rules().IsEmpty(),
		"el valor cero tiene que ser el seguro: sin reglas se ejecuta, nunca se revive")
}
