package fingerprint_test

// Vectores NORMATIVOS de la regla `sf-v1` (spec 27).
//
// SPEC-STEP-v1.md los reproduce en una tabla. Se dan con huellas de término
// FIJAS —no calculadas— para que la regla se pueda reproducir sin implementar
// las otras dos: lo que `sf-v1` hace es componer sobre dos cadenas.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domFingerprint "github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
)

func terminoFijo(t *testing.T, texto string) domFingerprint.Fingerprint {
	t.Helper()
	f, err := domFingerprint.Parse(texto)
	require.NoError(t, err)
	return f
}

func huellaDeStep(
	t *testing.T,
	declaracion, proyecto domFingerprint.Fingerprint,
	proyectoExcluido bool) string {

	t.Helper()
	f, err := domFingerprint.ComputeStepFingerprint(declaracion, proyecto, proyectoExcluido)
	require.NoError(t, err)
	return f.String()
}

func TestComputeStepFingerprint_Vectores(t *testing.T) {
	declaracion := terminoFijo(t, "pipe-v1:"+strings.Repeat("1", 64))
	otraDeclaracion := terminoFijo(t, "pipe-v1:"+strings.Repeat("2", 64))
	proyecto := terminoFijo(t, "v1:"+strings.Repeat("3", 64))
	// La MISMA declaración con la regla saltada a v2. No hace falta que `pipe-v2`
	// exista: la composición es sobre la cadena.
	declaracionV2 := terminoFijo(t, "pipe-v2:"+strings.Repeat("1", 64))

	cases := []struct {
		name     string
		pinea    string
		decl     domFingerprint.Fingerprint
		proyecto domFingerprint.Fingerprint
		excluido bool
		want     string
	}{
		{
			name:     "sólo la declaración",
			pinea:    "el step no vigila el proyecto: el término condicional va vacío",
			decl:     declaracion,
			excluido: true,
			want:     "sf-v1:73925e33d40bb78cfef488ff4f4ddd2a7700cbc483202e4549fccd36bcbe82f7",
		},
		{
			name:     "declaración y proyecto",
			pinea:    "§3.2: el mismo trabajo con y sin el proyecto son huellas distintas",
			decl:     declaracion,
			proyecto: proyecto,
			want:     "sf-v1:d32eb1c4e9aa19d36e059bb0207e98bd5f6e860659054072f5110db6b11a1ea6",
		},
		{
			name:     "otra declaración",
			pinea:    "cambiar la declaración cambia la huella",
			decl:     otraDeclaracion,
			excluido: true,
			want:     "sf-v1:ddcf8fe6824402dd789df9c350e2a8db71a78c2380343744ff2e5e65894537b9",
		},
		{
			name:     "la misma declaración con la regla en v2",
			pinea:    "§3.1: componer sobre la forma canónica COMPLETA, no sobre el hash pelado",
			decl:     declaracionV2,
			excluido: true,
			want:     "sf-v1:dae5ca9ee06cc0e9251bc11abc97b68669615d7da904c36a6295eda37abf4499",
		},
	}

	vistas := map[string]string{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := huellaDeStep(t, c.decl, c.proyecto, c.excluido)
			assert.Equal(t, c.want, got, c.pinea)

			if anterior, repetida := vistas[got]; repetida {
				t.Fatalf("mismo valor que el vector %q", anterior)
			}
			vistas[got] = c.name
		})
	}
}

// TestComputeStepFingerprint_UnSaltoDePipeV1CambiaTodasLasSfV1 es la propiedad
// que la disciplina de versiones de este motor compra: subir `pipe-v1` a `v2`
// invalida todas las `sf-v1` emitidas SIN TOCAR el código de `sf-v1`.
//
// Se conserva sólo mientras la composición sea sobre `String()`. Si algún día
// alguien compusiera sobre `Hash()`, este test es lo que se pone en rojo.
func TestComputeStepFingerprint_UnSaltoDePipeV1CambiaTodasLasSfV1(t *testing.T) {
	mismoHash := strings.Repeat("ab", 32)

	v1 := huellaDeStep(t, terminoFijo(t, "pipe-v1:"+mismoHash), domFingerprint.Fingerprint{}, true)
	v2 := huellaDeStep(t, terminoFijo(t, "pipe-v2:"+mismoHash), domFingerprint.Fingerprint{}, true)

	assert.NotEqual(t, v1, v2,
		"el mismo material identificado con dos reglas distintas no puede dar la misma huella")
}

// TestComputeStepFingerprint_ElTerminoDelProyectoEsCondicional es §3.2, y es la
// granularidad que la spec 10 §5.3 sacrificó: un step que crea un registro de
// contenedores no debe re-ejecutarse porque cambió el código de la aplicación.
func TestComputeStepFingerprint_ElTerminoDelProyectoEsCondicional(t *testing.T) {
	declaracion := terminoFijo(t, "pipe-v1:"+strings.Repeat("1", 64))
	unProyecto := terminoFijo(t, "v1:"+strings.Repeat("3", 64))
	otroProyecto := terminoFijo(t, "v1:"+strings.Repeat("4", 64))

	sinProyecto := huellaDeStep(t, declaracion, domFingerprint.Fingerprint{}, true)

	t.Run("el step que NO lo declara no se mueve al cambiar el código", func(t *testing.T) {
		assert.Equal(t, sinProyecto,
			huellaDeStep(t, declaracion, domFingerprint.Fingerprint{}, true))
	})

	t.Run("el que sí lo declara se mueve", func(t *testing.T) {
		assert.NotEqual(t,
			huellaDeStep(t, declaracion, unProyecto, false),
			huellaDeStep(t, declaracion, otroProyecto, false))
	})

	t.Run("declararlo y no declararlo son huellas distintas", func(t *testing.T) {
		assert.NotEqual(t, sinProyecto, huellaDeStep(t, declaracion, unProyecto, false))
	})
}

// TestComputeStepFingerprint_MaterialIncompletoEsError: una huella con un hueco
// es válida y COLISIONA con la de cualquier material al que le falte lo mismo, y
// esa colisión se manifiesta como un step que revive sin haberse ejecutado jamás.
//
// La guarda es de DOBLE SENTIDO a propósito: sin la segunda mitad, la forma
// canónica de un mismo step dependería de si alguien se acordó de limpiar el
// campo del proyecto.
func TestComputeStepFingerprint_MaterialIncompletoEsError(t *testing.T) {
	declaracion := terminoFijo(t, "pipe-v1:"+strings.Repeat("1", 64))
	proyecto := terminoFijo(t, "v1:"+strings.Repeat("3", 64))

	t.Run("sin declaración", func(t *testing.T) {
		_, err := domFingerprint.ComputeStepFingerprint(
			domFingerprint.Fingerprint{}, proyecto, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "declaración")
	})

	t.Run("sin proyecto y sin declararlo excluido", func(t *testing.T) {
		_, err := domFingerprint.ComputeStepFingerprint(
			declaracion, domFingerprint.Fingerprint{}, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "proyecto")
	})

	t.Run("excluyendo el proyecto y a la vez trayéndolo", func(t *testing.T) {
		_, err := domFingerprint.ComputeStepFingerprint(declaracion, proyecto, true)
		require.Error(t, err)
	})
}

// TestComputeStepFingerprint_UnRegistroCkV1NoRevive es la razón por la que la
// spec 27 no escribe migrador: la comparación es de cadenas COMPLETAS con
// prefijo, así que ningún registro de la época anterior puede acertar una huella
// de ésta.
func TestComputeStepFingerprint_UnRegistroCkV1NoRevive(t *testing.T) {
	huella := huellaDeStep(t,
		terminoFijo(t, "pipe-v1:"+strings.Repeat("1", 64)), domFingerprint.Fingerprint{}, true)

	assert.True(t, strings.HasPrefix(huella, "sf-v1:"))
	assert.NotEqual(t, "ck-v1:"+strings.TrimPrefix(huella, "sf-v1:"), huella)
}

func TestComputeStepFingerprint_ElPrefijoEsObligatorio(t *testing.T) {
	f, err := domFingerprint.ComputeStepFingerprint(
		terminoFijo(t, "pipe-v1:"+strings.Repeat("1", 64)), domFingerprint.Fingerprint{}, true)
	require.NoError(t, err)

	assert.Equal(t, domFingerprint.StepVersion, f.Version())
	assert.Equal(t, "sf-v1", f.Version())
}
