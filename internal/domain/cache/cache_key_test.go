package cache_test

// Vectores y sensibilidades de la clave de caché (SPEC-v1.md de este paquete).
//
// Lo que estos tests defienden no es un formato: es que **el ambiente no se
// pueda volver a caer de la clave**. Hasta la spec 10, tres de las cuatro claves
// anteriores no lo llevaban, y que `prod` no se saltara tras un despliegue a
// `sand` dependía de una coincidencia que nadie escribió con esa intención.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
)

func huellaFija(t *testing.T, version, digito string) fingerprint.Fingerprint {
	t.Helper()
	huella, err := fingerprint.Parse(version + ":" + strings.Repeat(digito, 32))
	require.NoError(t, err)
	return huella
}

// materialBase es el de §6 de la especificación.
func materialBase(t *testing.T) cache.Material {
	t.Helper()
	return cache.Material{
		Subject:      "https://vex.test/acme/demo-app",
		Pipeline:     "https://vex.test/acme/pipelinecode",
		Scope:        "sand",
		Step:         "supply",
		Instructions: huellaFija(t, fingerprint.InstructionsVersion, "11"),
		Variables:    huellaFija(t, fingerprint.VariablesVersion, "22"),
		Code:         huellaFija(t, fingerprint.Version, "33"),
	}
}

func clave(t *testing.T, material cache.Material) string {
	t.Helper()
	key, err := cache.NewCacheKey(material)
	require.NoError(t, err)
	return key.String()
}

// --- §6 vectores ------------------------------------------------------------

func TestNewCacheKey_Vectores(t *testing.T) {
	base := materialBase(t)

	prod := materialBase(t)
	prod.Scope = "prod"

	assert.Equal(t,
		"ck-v1:6d12da1d013dd8ea8cefb1d095900471afba22828ec4cd3dcc2c972718545126",
		clave(t, base), "vector 1 de SPEC-v1.md §6")
	assert.Equal(t,
		"ck-v1:a6ba010f5849846aa8f44bbff1acd1aacb501a131e89cd40698a52525f0ab241",
		clave(t, prod), "vector 2 de SPEC-v1.md §6")
}

// --- §7.2 las siete dimensiones ---------------------------------------------

// Siete comprobaciones, no una: cada dimensión del material tiene que mover la
// clave por sí sola. Si alguna deja de hacerlo, dos ejecuciones distintas
// compartirían entrada y una de ellas se saltaría sin haberse ejecutado.
func TestNewCacheKey_CadaDimensionMueveLaClave(t *testing.T) {
	base := clave(t, materialBase(t))

	mutaciones := map[string]func(*cache.Material){
		"subject":  func(m *cache.Material) { m.Subject = "https://vex.test/acme/otra-app" },
		"pipeline": func(m *cache.Material) { m.Pipeline = "https://vex.test/acme/otro-pipelinecode" },
		"scope":    func(m *cache.Material) { m.Scope = "prod" },
		"step":     func(m *cache.Material) { m.Step = "deploy" },
		"instructions": func(m *cache.Material) {
			m.Instructions = huellaFija(t, fingerprint.InstructionsVersion, "aa")
		},
		"variables": func(m *cache.Material) {
			m.Variables = huellaFija(t, fingerprint.VariablesVersion, "bb")
		},
		"code": func(m *cache.Material) {
			m.Code = huellaFija(t, fingerprint.Version, "cc")
		},
	}

	for nombre, mutar := range mutaciones {
		t.Run(nombre, func(t *testing.T) {
			material := materialBase(t)
			mutar(&material)
			assert.NotEqual(t, base, clave(t, material))
		})
	}
}

// EL test de la spec 10: el ambiente está en la clave POR DERECHO PROPIO.
//
// La variante que hoy rompería: aunque `environment` desapareciera del material
// de la huella de variables —una decisión que parece razonable: «es volátil»—,
// dos ámbitos siguen dando claves distintas. Antes de la spec 10, esa misma
// decisión hacía que desplegar a `sand` dejara escrito «sin cambios» para `prod`.
func TestNewCacheKey_ElAmbienteNoSePuedeCaerDeLaClave(t *testing.T) {
	sand := materialBase(t)
	sand.Scope = "sand"

	prod := materialBase(t)
	prod.Scope = "prod"

	compartido := materialBase(t)
	compartido.Scope = "shared"

	// Los tres materiales son IDÉNTICOS salvo el ámbito: mismas tres huellas,
	// mismo proyecto, mismo pipeline, mismo paso.
	claves := []string{clave(t, sand), clave(t, prod), clave(t, compartido)}

	assert.NotEqual(t, claves[0], claves[1], "sand y prod no pueden compartir entrada")
	assert.NotEqual(t, claves[0], claves[2])
	assert.NotEqual(t, claves[1], claves[2])
}

// Corrige la asimetría de la regla de tiempo, que no llevaba el pipeline: dos
// pipelines sobre el mismo proyecto compartían TTL.
func TestNewCacheKey_DosPipelinesDelMismoProyectoNoComparteEntrada(t *testing.T) {
	uno := materialBase(t)
	otro := materialBase(t)
	otro.Pipeline = "https://vex.test/acme/pipelinecode-critical"

	assert.NotEqual(t, clave(t, uno), clave(t, otro))
}

// --- §7.3 material incompleto ⇒ error, no clave -----------------------------

func TestNewCacheKey_MaterialIncompletoEsUnError(t *testing.T) {
	vaciar := map[string]func(*cache.Material){
		"subject":      func(m *cache.Material) { m.Subject = "" },
		"pipeline":     func(m *cache.Material) { m.Pipeline = "" },
		"scope":        func(m *cache.Material) { m.Scope = "" },
		"step":         func(m *cache.Material) { m.Step = "" },
		"instructions": func(m *cache.Material) { m.Instructions = fingerprint.Fingerprint{} },
		"variables":    func(m *cache.Material) { m.Variables = fingerprint.Fingerprint{} },
		"code":         func(m *cache.Material) { m.Code = fingerprint.Fingerprint{} },
	}

	for nombre, romper := range vaciar {
		t.Run("sin "+nombre, func(t *testing.T) {
			material := materialBase(t)
			romper(&material)

			key, err := cache.NewCacheKey(material)

			require.Error(t, err, "un material con un hueco produciría una clave que colisiona")
			assert.Contains(t, err.Error(), nombre, "el error nombra la dimensión que falta")
			assert.True(t, key.IsZero(), "y no se devuelve una clave degradada")
		})
	}
}

// --- §3.1bis el término condicional -----------------------------------------

// La séptima dimensión es CONDICIONAL desde la spec 15: un step que declara
// `state_changed: [pipeline]` no vigila el código del proyecto, y su material lo
// dice explícitamente.
//
// Lo que este caso fija es que la excepción no reabre la puerta que §3.1 cerró:
// el hueco tiene que estar DECLARADO por los dos lados, y una clave con el
// código fuera no puede coincidir con una que lo lleva.
func TestNewCacheKey_ElCodigoDelProyectoEsCondicionalYDeclarado(t *testing.T) {
	sinCodigo := materialBase(t)
	sinCodigo.Code = fingerprint.Fingerprint{}
	sinCodigo.CodeExcluded = true

	require.NoError(t, sinCodigo.Validate())
	assert.NotEqual(t, clave(t, materialBase(t)), clave(t, sinCodigo),
		"dos alcances distintos no pueden compartir identidad")

	t.Run("excluirlo y traerlo a la vez es un error", func(t *testing.T) {
		contradictorio := materialBase(t)
		contradictorio.CodeExcluded = true

		_, err := cache.NewCacheKey(contradictorio)

		require.Error(t, err,
			"si no, la forma canónica dependería de si alguien se acordó de limpiar el campo")
	})

	// La razón por la que esto NO es una `ck-v2`: la regla de composición no
	// cambia, y el dominio de materiales se amplía de forma INYECTIVA. Ningún
	// material expresable antes cambia de clave.
	t.Run("los vectores de §6 no se mueven", func(t *testing.T) {
		assert.Equal(t,
			"ck-v1:6d12da1d013dd8ea8cefb1d095900471afba22828ec4cd3dcc2c972718545126",
			clave(t, materialBase(t)))
	})
}

// --- §7.4 el versionado sale gratis -----------------------------------------

// La clave se compone sobre la forma canónica COMPLETA de cada huella, no sobre
// el hash pelado. De ahí sale que un salto a `v2` de cualquiera de las tres
// reglas invalide todas las claves emitidas sin una línea de código extra.
func TestNewCacheKey_UnSaltoDeVersionDeUnaHuellaInvalidaLaClave(t *testing.T) {
	base := materialBase(t)

	for _, caso := range []struct {
		nombre string
		mutar  func(*cache.Material)
	}{
		{"árbol", func(m *cache.Material) { m.Code = huellaFija(t, "v2", "33") }},
		{"instrucciones", func(m *cache.Material) { m.Instructions = huellaFija(t, "inst-v2", "11") }},
		{"variables", func(m *cache.Material) { m.Variables = huellaFija(t, "vars-v2", "22") }},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			v2 := materialBase(t)
			caso.mutar(&v2)

			assert.NotEqual(t, clave(t, base), clave(t, v2),
				"MISMO hash, otra versión de la regla: la clave tiene que moverse")
		})
	}
}

// --- §7.5 la composición es una función pura --------------------------------

func TestNewCacheKey_EsUnaFuncionPura(t *testing.T) {
	primera := clave(t, materialBase(t))
	segunda := clave(t, materialBase(t))

	assert.Equal(t, primera, segunda)
	assert.Regexp(t, `^ck-v1:[0-9a-f]{64}$`, primera)
}

// --- inyectividad -----------------------------------------------------------

// Ningún valor puede simular un salto de línea y hacerse pasar por otro reparto
// de campos (§5.1). Es lo que esta regla NO hereda de la huella del árbol, que
// dejó esa ambigüedad congelada.
func TestNewCacheKey_UnValorNoPuedeSimularElRepartoDeCampos(t *testing.T) {
	inyectado := materialBase(t)
	inyectado.Subject = "https://vex.test/acme/demo-app\"\n\"https://vex.test/acme/pipelinecode"
	inyectado.Pipeline = "x"

	assert.NotEqual(t, clave(t, materialBase(t)), clave(t, inyectado))
}

// --- forma externa ----------------------------------------------------------

func TestCacheKey_FormaExterna(t *testing.T) {
	key, err := cache.NewCacheKey(materialBase(t))
	require.NoError(t, err)

	t.Run("lleva la versión de la regla de composición", func(t *testing.T) {
		assert.Equal(t, cache.KeyVersion, key.Version())
		assert.Len(t, key.Hash(), 64)
		assert.Equal(t, key.Version()+":"+key.Hash(), key.String())
	})

	t.Run("va y vuelve intacta", func(t *testing.T) {
		recuperada, err := cache.ParseCacheKey(key.String())
		require.NoError(t, err)
		assert.True(t, key.Equals(recuperada))
	})

	t.Run("la clave cero no es una clave", func(t *testing.T) {
		var cero cache.CacheKey
		assert.True(t, cero.IsZero())
		assert.Equal(t, "", cero.String())
		assert.False(t, cero.Equals(key))
	})

	t.Run("dos versiones distintas nunca son iguales", func(t *testing.T) {
		otraVersion, err := cache.ParseCacheKey("ck-v2:" + key.Hash())
		require.NoError(t, err)
		assert.False(t, key.Equals(otraVersion), "mismo hash, otra regla de composición")
	})

	t.Run("una cadena que no tiene la forma se rechaza", func(t *testing.T) {
		for _, malformada := range []string{
			"sin-separador",
			":" + key.Hash(),
			"ck-v1:corto",
			"ck-v1:" + strings.Repeat("Z", 64),
		} {
			_, err := cache.ParseCacheKey(malformada)
			assert.Error(t, err, malformada)
		}
	})
}
