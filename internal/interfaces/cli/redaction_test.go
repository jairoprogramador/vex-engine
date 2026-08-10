package cli_test

// «Los valores nunca entran al registro» (spec 20 §7), verificado contra el
// cableado REAL.
//
// Es la aserción que da nombre a la spec, y hasta aquí era una aspiración: el
// registro emitía un `sha256` desnudo de cada valor, que para un parámetro de
// baja entropía —`REPLICAS=3`— es una búsqueda en una tabla y no un secreto.
//
// Los casos vienen en pares deliberados, porque las dos propiedades que la spec
// cierra son OPUESTAS y las dos tienen que cumplirse a la vez:
//
//	content_id                    → SHA-256 SIN SAL. Dos organizaciones con el
//	                                mismo contenido obtienen la misma identidad,
//	                                que es su razón de existir.
//	parameter_resolved{digest}    → HMAC con clave POR PROYECTO. Dos proyectos
//	                                con el mismo valor obtienen resúmenes
//	                                distintos, porque uno suelto es reversible.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// ── La aserción que da nombre a la spec ─────────────────────────────────────

// NINGÚN HECHO LLEVA EL VALOR DE UNA VARIABLE, en ningún campo de ningún tipo.
//
// El caso anterior (`TestHechos_CadaParametroResueltoDejaElSuyo`) miraba los
// campos de `parameter_resolved` contra UN valor. Éste recorre TODOS los hechos
// contra TODOS los valores que la ejecución produjo o declaró, que es lo que la
// §7 pide: la promesa es del registro entero, no de un tipo de hecho.
func TestRedaccion_NingunHechoLlevaElValorDeUnaVariable(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	valores := h.valoresDeLaEjecucion()
	require.NotEmpty(t, valores, "sin valores que buscar, el caso no afirmaría nada")

	for _, hecho := range h.hechos() {
		for campo, valor := range hecho.Payload {
			texto, ok := valor.(string)
			if !ok {
				continue
			}
			for nombre, esperado := range valores {
				assert.NotContains(t, texto, esperado,
					"el hecho '%s' lleva el valor de '%s' en el campo '%s'",
					hecho.Type, nombre, campo)
			}
		}
	}
}

// ── HMAC vs SHA-256: las dos propiedades opuestas ───────────────────────────

// DOS PROYECTOS CON EL MISMO VALOR PRODUCEN RESÚMENES DISTINTOS.
//
// **Es la razón de ser de §5.2.** Los dos comparten destino, así que comparten
// el secreto local: lo único que los separa es el identificador del proyecto. Con
// un SHA-256 desnudo los dos resúmenes serían idénticos —y triviales de
// invertir—, así que este caso no distinguiría nada.
func TestRedaccion_DosProyectosNoCompartenElResumenDeUnValor(t *testing.T) {
	uno := newHarness(t)
	otro := newHarness(t)
	otro.compartiendoElDestinoCon(uno)

	require.Equal(t, cli.ExitSucceeded, uno.run().exitCode)
	require.Equal(t, cli.ExitSucceeded, otro.run().exitCode)

	// El mismo comando, el mismo stdout, el mismo valor extraído: `demo-app`.
	assert.NotEqual(t, uno.digestDe("artifact_name"), otro.digestDe("artifact_name"),
		"la clave es POR PROYECTO, y sin eso un valor de baja entropía es reversible")
}

// Y LA PROPIEDAD OPUESTA, QUE TAMBIÉN DEBE CUMPLIRSE: dos instalaciones con el
// mismo contenido derivan el MISMO `content_id`.
//
// Las dos máquinas tienen destinos distintos —luego secretos distintos— así que
// el mismo caso mide las dos mitades de D-A10 de una vez: la identidad viaja sin
// sal a propósito, porque compararla entre organizaciones es para lo que existe;
// el resumen de un valor suelto no puede.
func TestRedaccion_LaIdentidadEsComparableYElResumenNo(t *testing.T) {
	una := newHarness(t)
	require.Equal(t, cli.ExitSucceeded, una.run().exitCode)

	otra := una.otraMaquina()
	require.Equal(t, cli.ExitSucceeded, otra.run().exitCode)

	assert.Equal(t, una.elObjeto().ContentID, otra.elObjeto().ContentID,
		"`content_id` va SIN SAL: es irreversible por composición y comparable entre organizaciones")

	assert.NotEqual(t, una.digestDe("artifact_name"), otra.digestDe("artifact_name"),
		"el resumen de un valor suelto sí lleva sal, y por eso no se compara entre instalaciones")
}

// EL RESUMEN ES ESTABLE DENTRO DEL PROYECTO. Es la única propiedad para la que
// existe: responder «¿corrió con lo mismo que ayer?».
//
// La segunda corrida cambia un byte del proyecto para que los steps vuelvan a
// ejecutarse —sin eso revivirían y no habría segundo `parameter_resolved`— pero
// el valor extraído es el mismo, así que su resumen tiene que serlo.
func TestRedaccion_ElResumenEsElMismoEntreEjecucionesDelMismoProyecto(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	primera := h.digestDe("artifact_name")

	h.writeProjectFile("src/app.txt", "v2\n")
	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)
	segunda := h.digestDe("artifact_name")

	require.NotEmpty(t, primera)
	assert.Equal(t, primera, segunda)
}

// El resumen lleva su convención delante, y eso es lo que impide que un digest
// de la spec 19 y uno de la 20 se comparen como si fueran lo mismo: no dirían
// «el valor cambió», que sería mentira, sino «esto no es comparable».
func TestRedaccion_ElResumenDiceConQueConvencionSeCalculo(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	assert.Regexp(t, "^"+record.DigestVersion+":[0-9a-f]{64}$", h.digestDe("artifact_name"))
}

// ── Dónde cae la frontera, decidido y hecho observable ──────────────────────

// LA PROMESA ES SOBRE VALORES RESUELTOS, NO SOBRE LITERALES DEL PIPELINECODE.
//
// Es la decisión que las specs 17 §9.1 y 18 dejaron abierta aquí, y hay que
// tomarla ahora porque cambiarla después sería un salto a `cnt-v2` con
// `content_id` ya emitidos, que son permanentes:
//
//	El OBJETO lleva la DECLARACIÓN de cada parámetro, y la declaración de un
//	literal ES su valor entrecomillado. Sale de la organización cuando la spec
//	21 empuje el objeto. Se acepta: vive en el repositorio del pipelinecode,
//	versionado en git y visible para cualquiera con acceso a él, y quitarlo del
//	material no es una opción disponible —dos pipelinecode que sólo difieren en
//	el valor de un literal declaran ejecutar cosas distintas, y sin el valor
//	dentro del material el `content_id` no los distinguiría—.
//
//	El REGISTRO no lleva ningún valor RESUELTO. Ni el literal ya interpolado, ni
//	lo extraído del stdout, ni lo leído del almacén.
//
// La consecuencia práctica, que la guía ya dice y aquí queda anclada: un secreto
// no se declara en `variables/`. Llega por el entorno de la shell (`$VAR`), que
// es la única vía que no pasa por el motor.
func TestRedaccion_ElObjetoLlevaElLiteralYElRegistroNoLlevaElValor(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	// La declaración entra entera en la identidad —es la mitad de la regla que
	// `TestRunCommand_ElObjetoLlevaLaDeclaracionDeCadaStep` fija— y aquí importa
	// como el LADO CONOCIDO de la frontera, no como un descuido.
	supply := h.elObjeto().Steps[1]
	require.Len(t, supply.Parameters, 1)
	assert.Contains(t, supply.Parameters[0].Declaration, "vexsand")

	// Y el otro lado: `acr_name` se resolvió a `vexsand-demo-app` y ningún hecho
	// lo lleva. Lo que viaja es el nombre, la procedencia y un resumen con sal.
	for _, hecho := range h.hechos() {
		for _, valor := range hecho.Payload {
			texto, ok := valor.(string)
			if !ok {
				continue
			}
			assert.NotContains(t, texto, "vexsand-demo-app",
				"el hecho '%s' lleva un valor resuelto", hecho.Type)
		}
	}
}

// Un campo obligatorio ausente lo sigue diagnosticando quien los nombra los
// nueve. Derivar la clave del resumen antes de esa validación cambiaría
// «project id is required» por un mensaje sobre resúmenes, que es peor
// diagnóstico para el mismo defecto.
func TestRedaccion_UnProyectoSinIdSigueDiagnosticandoseComoCampoAusente(t *testing.T) {
	h := newHarness(t)

	result := h.run(withProjectId(""))

	assert.Equal(t, cli.ExitFailed, result.exitCode)
	assert.Contains(t, result.stderr, "project id is required")
	assert.Empty(t, h.ranSteps())
}

// ── El secreto no es registro ───────────────────────────────────────────────

// LA CLAVE VIVE FUERA DEL REGISTRO (§6). No está en `state/`, ni en `cache/`, ni
// en `lineage/`, ni en el área de trabajo de donde la spec 21 empujará los
// objetos y los hechos: está en `keys/`, que no se sincroniza a ningún sitio.
//
// Escribirlo como test y no como comentario es lo que convierte «fuera del
// registro» en una propiedad y no en una intención.
func TestRedaccion_ElSecretoDelResumenNoEsParteDelRegistro(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, cli.ExitSucceeded, h.run().exitCode)

	secreto := h.rutaDelSecreto()
	require.FileExists(t, secreto)

	for _, dentro := range []string{h.statePath(), h.cachePath(), h.staging} {
		assert.NotContains(t, secreto, dentro,
			"el secreto no puede caer bajo nada que se sincronice")
	}
}
