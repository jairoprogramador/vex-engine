package deployment_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
	"github.com/jairoprogramador/vex-engine/internal/domain/step"
)

// El material base de los vectores de `SPEC-CONTENT-v1.md` §7. Las huellas son
// literales fijos —no calculados— para que este documento se pueda validar sin
// reimplementar antes las tres reglas de huella.
const (
	sujetoBase    = "https://vex.test/acme/demo-app"
	operacionBase = "deploy"
	destinoBase   = "sand"
)

type materialDeContenido struct {
	subject     string
	operation   string
	destination string
	project     fingerprint.Fingerprint
	pipeline    fingerprint.Fingerprint
	format      deployment.Format
	steps       []deployment.StepContent
}

func materialBase(t *testing.T) materialDeContenido {
	t.Helper()

	formato, err := deployment.NewFormat(2, true)
	require.NoError(t, err)

	return materialDeContenido{
		subject:     sujetoBase,
		operation:   operacionBase,
		destination: destinoBase,
		project:     huella(t, fingerprint.Version, "33"),
		pipeline:    huella(t, fingerprint.Version, "44"),
		format:      formato,
		steps:       []deployment.StepContent{stepTest(t), stepSupply(t)},
	}
}

// stepTest es un step SIN `config.yaml`: no declara ámbito ni reglas, se ejecuta
// siempre y no escribe registro (spec 13 §5.3).
func stepTest(t *testing.T) deployment.StepContent {
	t.Helper()

	contenido, err := deployment.NewStepContent(
		"01-test",
		step.NoStepConfig(),
		huella(t, fingerprint.InstructionsVersion, "11"),
		nil)
	require.NoError(t, err)
	return contenido
}

// stepSupply declara ámbito de PROYECTO —su material no depende del destino— y
// una regla que no vigila el código de la aplicación.
func stepSupply(t *testing.T) deployment.StepContent {
	t.Helper()

	regla, err := step.NewStateChangedRule([]string{step.StateSourcePipeline})
	require.NoError(t, err)
	reglas, err := step.NewRuleSet(regla)
	require.NoError(t, err)
	config, err := step.NewStepConfig(step.NewProjectScope(), reglas)
	require.NoError(t, err)

	literal, err := step.NewLiteralDeclaration("acr_name", "vexacr")
	require.NoError(t, err)
	producida, err := step.NewStepOutputDeclaration("image", "01-test", "image")
	require.NoError(t, err)

	contenido, err := deployment.NewStepContent(
		"02-supply",
		config,
		huella(t, fingerprint.InstructionsVersion, "22"),
		[]step.VariableDeclaration{literal, producida})
	require.NoError(t, err)
	return contenido
}

func (m materialDeContenido) componer(t *testing.T) deployment.Content {
	t.Helper()

	contenido, err := m.intentar()
	require.NoError(t, err)
	return contenido
}

func (m materialDeContenido) intentar() (deployment.Content, error) {
	subject, err := deployment.NewSubject(m.subject)
	if err != nil {
		return deployment.Content{}, err
	}
	operation, err := deployment.NewOperation(m.operation)
	if err != nil {
		return deployment.Content{}, err
	}
	destination, err := deployment.NewDestination(m.destination)
	if err != nil {
		return deployment.Content{}, err
	}
	source, err := deployment.NewSource(m.project, m.pipeline)
	if err != nil {
		return deployment.Content{}, err
	}
	return deployment.NewContent(subject, operation, destination, source, m.format, m.steps)
}

// ── Determinismo y sensibilidad ─────────────────────────────────────────────

func TestContentID_EsDeterminista(t *testing.T) {
	uno := materialBase(t).componer(t)
	otro := materialBase(t).componer(t)

	assert.True(t, uno.ID().Equals(otro.ID()),
		"el mismo Content compuesto dos veces produce el mismo content_id")
	assert.Equal(t, uno.Canonical(), otro.Canonical())
	assert.Equal(t, deployment.ContentIDVersion, uno.ID().Version())
}

func TestContentID_UnByteDistintoEnCualquierCampoLoAltera(t *testing.T) {
	base := materialBase(t).componer(t).ID()

	casos := map[string]func(m *materialDeContenido){
		"sujeto": func(m *materialDeContenido) {
			m.subject = "https://vex.test/acme/otra-app"
		},
		"operación": func(m *materialDeContenido) {
			m.operation = "supply"
		},
		"destino": func(m *materialDeContenido) {
			m.destination = "prod"
		},
		"huella del proyecto": func(m *materialDeContenido) {
			m.project = huella(t, fingerprint.Version, "55")
		},
		"huella del pipelinecode": func(m *materialDeContenido) {
			m.pipeline = huella(t, fingerprint.Version, "66")
		},
		"versión del formato": func(m *materialDeContenido) {
			formato, err := deployment.NewFormat(1, true)
			require.NoError(t, err)
			m.format = formato
		},
		"el manifiesto se declara o no": func(m *materialDeContenido) {
			formato, err := deployment.NewFormat(2, false)
			require.NoError(t, err)
			m.format = formato
		},
		"un step menos": func(m *materialDeContenido) {
			m.steps = m.steps[:1]
		},
		"el orden de los steps": func(m *materialDeContenido) {
			m.steps = []deployment.StepContent{m.steps[1], m.steps[0]}
		},
	}

	for nombre, mutar := range casos {
		t.Run(nombre, func(t *testing.T) {
			material := materialBase(t)
			mutar(&material)

			assert.False(t, base.Equals(material.componer(t).ID()))
		})
	}
}

func TestContentID_ComponeSobreLaFormaCanonicaCompletaDeLaHuella(t *testing.T) {
	// El mismo hash bajo otra versión de la regla del árbol tiene que cambiar el
	// content_id. Si se compusiera sobre el hash pelado serían IGUALES, y dos
	// objetos distintos compartirían una identidad PERMANENTE.
	base := materialBase(t)
	v2 := materialBase(t)
	v2.project = huella(t, "v2", "33")

	assert.False(t, base.componer(t).ID().Equals(v2.componer(t).ID()))
}

// ── Las exclusiones: el test que protege el direccionamiento por contenido ──

func TestContentID_NoLlevaTiempoNiActorNiRunnerNiPadre(t *testing.T) {
	// La exclusión está sostenida por el TIPO: `Content` no tiene dónde poner un
	// instante, un actor, un runner ni un padre — el actor y el runner viven en
	// `attempt_started` y el padre en `DeploymentID`. Lo que este test comprueba
	// es la consecuencia observable: el mismo contenido en dos posiciones
	// distintas de la historia conserva su identidad.
	contenido := materialBase(t).componer(t)

	primero, err := deployment.DeploymentIDOf(contenido.ID(), deployment.DeploymentID{})
	require.NoError(t, err)
	segundo, err := deployment.DeploymentIDOf(contenido.ID(), primero)
	require.NoError(t, err)

	assert.False(t, primero.Equals(segundo), "la POSICIÓN cambia")
	assert.True(t, contenido.ID().Equals(materialBase(t).componer(t).ID()),
		"la INTENCIÓN no")
}

func TestContentID_EntraLaDeclaracionNuncaElValor(t *testing.T) {
	// Dos ejecuciones que resuelven valores distintos para la MISMA declaración
	// producen el mismo content_id: en el objeto no hay ningún valor resuelto,
	// sólo la declaración de cómo obtenerlo. Es la precondición dura de todo el
	// registro — un valor de runtime no se puede saber por adelantado.
	conUnaFuente := func(from string) deployment.Content {
		declaracion, err := step.NewStepOutputDeclaration("image", from, "image")
		require.NoError(t, err)
		stepContent, err := deployment.NewStepContent(
			"02-supply",
			step.NoStepConfig(),
			huella(t, fingerprint.InstructionsVersion, "22"),
			[]step.VariableDeclaration{declaracion})
		require.NoError(t, err)

		material := materialBase(t)
		material.steps = []deployment.StepContent{stepContent}
		return material.componer(t)
	}

	assert.True(t, conUnaFuente("01-test").ID().Equals(conUnaFuente("01-test").ID()))
	assert.False(t, conUnaFuente("01-test").ID().Equals(conUnaFuente("01-build").ID()),
		"dos pipelinecode que sólo difieren en un 'from' declaran cosas distintas")
}

func TestContentID_UnLiteralDistintoEsOtroContenido(t *testing.T) {
	// La decisión de esta spec: `parameters` lleva TODAS las declaraciones, no
	// sólo las que declaran fuente. El objeto se compone antes de ejecutar, así
	// que un literal que no entrara aquí no entraría por ninguna otra vía.
	conLiteral := func(valor string) deployment.Content {
		declaracion, err := step.NewLiteralDeclaration("acr_name", valor)
		require.NoError(t, err)
		stepContent, err := deployment.NewStepContent(
			"02-supply",
			step.NoStepConfig(),
			huella(t, fingerprint.InstructionsVersion, "22"),
			[]step.VariableDeclaration{declaracion})
		require.NoError(t, err)

		material := materialBase(t)
		material.steps = []deployment.StepContent{stepContent}
		return material.componer(t)
	}

	assert.False(t, conLiteral("vexacr").ID().Equals(conLiteral("otroacr").ID()))
}

func TestContentID_ElOrdenDeLosParametrosNoLoMueve(t *testing.T) {
	// Los parámetros son un mapa por nombre en el archivo: el orden en que se
	// leyeron es circunstancia del parser, no contenido.
	uno, err := step.NewLiteralDeclaration("a", "1")
	require.NoError(t, err)
	otro, err := step.NewLiteralDeclaration("b", "2")
	require.NoError(t, err)

	conOrden := func(declaraciones ...step.VariableDeclaration) deployment.Content {
		stepContent, err := deployment.NewStepContent(
			"01-test",
			step.NoStepConfig(),
			huella(t, fingerprint.InstructionsVersion, "11"),
			declaraciones)
		require.NoError(t, err)

		material := materialBase(t)
		material.steps = []deployment.StepContent{stepContent}
		return material.componer(t)
	}

	assert.True(t, conOrden(uno, otro).ID().Equals(conOrden(otro, uno).ID()))
}

// ── El sub-bloque independiente del destino ────────────────────────────────

func TestContent_ElSubBloqueDeProyectoNoDependeDelDestino(t *testing.T) {
	sand := materialBase(t)
	prod := materialBase(t)
	prod.destination = "prod"

	enSand := sand.componer(t).ProjectScopedSteps()
	enProd := prod.componer(t).ProjectScopedSteps()

	require.Len(t, enSand, 1, "sólo 02-supply declara ámbito de proyecto")
	require.Len(t, enProd, 1)

	assert.Equal(t, enSand[0].StepID(), enProd[0].StepID())
	assert.Equal(t, canonicalDeSteps(enSand), canonicalDeSteps(enProd),
		"el material de un step de ámbito de proyecto es el mismo se despliegue donde se despliegue")

	assert.False(t, sand.componer(t).ID().Equals(prod.componer(t).ID()),
		"el OBJETO sí depende del destino: lo que no depende es este sub-bloque")
}

// canonicalDeSteps recompone el material de un sub-bloque a través de un
// `Content` idéntico salvo por esos steps, que es la única forma de observar la
// forma canónica de un step desde fuera del paquete.
func canonicalDeSteps(steps []deployment.StepContent) string {
	nombres := make([]string, 0, len(steps))
	for _, contenido := range steps {
		nombres = append(nombres, contenido.StepID()+":"+contenido.Instructions().String())
		for _, parametro := range contenido.Parameters() {
			nombres = append(nombres, parametro.Name()+"="+parametro.Canonical())
		}
		nombres = append(nombres,
			contenido.Config().Scope().String(),
			contenido.Config().Rules().Canonical())
	}
	return strings.Join(nombres, "|")
}

// ── Material incompleto ⇒ error ────────────────────────────────────────────

func TestContent_MaterialIncompletoEsUnErrorNoUnaIdentidadDegradada(t *testing.T) {
	casos := map[string]func(m *materialDeContenido){
		"sin sujeto":    func(m *materialDeContenido) { m.subject = "" },
		"sin operación": func(m *materialDeContenido) { m.operation = "" },
		"sin destino":   func(m *materialDeContenido) { m.destination = "" },
		"sin huella del proyecto": func(m *materialDeContenido) {
			m.project = fingerprint.Fingerprint{}
		},
		"sin huella del pipelinecode": func(m *materialDeContenido) {
			m.pipeline = fingerprint.Fingerprint{}
		},
		"sin formato": func(m *materialDeContenido) { m.format = deployment.Format{} },
		"sin steps":   func(m *materialDeContenido) { m.steps = nil },
		"con un step sin material": func(m *materialDeContenido) {
			m.steps = []deployment.StepContent{{}}
		},
		"con el mismo step dos veces": func(m *materialDeContenido) {
			m.steps = []deployment.StepContent{stepTest(t), stepTest(t)}
		},
	}

	for nombre, mutar := range casos {
		t.Run(nombre, func(t *testing.T) {
			material := materialBase(t)
			mutar(&material)

			_, err := material.intentar()
			require.Error(t, err)
		})
	}
}

func TestContent_NoSeComponeConValoresCero(t *testing.T) {
	// La guarda no está sólo en los value objects: `NewContent` vuelve a
	// comprobarlo porque nada impide construirlo con un valor cero, y un objeto
	// con un hueco produce una identidad válida que COLISIONA con la de
	// cualquier otro al que le falte lo mismo.
	sujeto, err := deployment.NewSubject(sujetoBase)
	require.NoError(t, err)
	operacion, err := deployment.NewOperation(operacionBase)
	require.NoError(t, err)
	destino, err := deployment.NewDestination(destinoBase)
	require.NoError(t, err)
	fuente, err := deployment.NewSource(
		huella(t, fingerprint.Version, "33"), huella(t, fingerprint.Version, "44"))
	require.NoError(t, err)
	formato, err := deployment.NewFormat(2, true)
	require.NoError(t, err)
	steps := []deployment.StepContent{stepTest(t)}

	casos := map[string]func() (deployment.Content, error){
		"sujeto cero": func() (deployment.Content, error) {
			return deployment.NewContent(
				deployment.Subject{}, operacion, destino, fuente, formato, steps)
		},
		"operación cero": func() (deployment.Content, error) {
			return deployment.NewContent(
				sujeto, deployment.Operation{}, destino, fuente, formato, steps)
		},
		"destino cero": func() (deployment.Content, error) {
			return deployment.NewContent(
				sujeto, operacion, deployment.Destination{}, fuente, formato, steps)
		},
		"fuente cero": func() (deployment.Content, error) {
			return deployment.NewContent(
				sujeto, operacion, destino, deployment.Source{}, formato, steps)
		},
		"formato cero": func() (deployment.Content, error) {
			return deployment.NewContent(
				sujeto, operacion, destino, fuente, deployment.Format{}, steps)
		},
	}

	for nombre, componer := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := componer()
			require.Error(t, err)
		})
	}
}

func TestContent_ElValorCeroNoTieneIdentidad(t *testing.T) {
	var contenido deployment.Content
	assert.True(t, contenido.ID().IsZero())
}

// ── La marca de incompleto ─────────────────────────────────────────────────

func TestContent_UnPipelinecodeSinManifiestoSeEmiteMARCADO(t *testing.T) {
	// Omitir el objeto perdería historia; emitirlo como completo sería mentir.
	sinManifiesto := materialBase(t)
	formato, err := deployment.NewFormat(1, false)
	require.NoError(t, err)
	sinManifiesto.format = formato

	contenido := sinManifiesto.componer(t)
	assert.False(t, contenido.IsComplete())
	assert.False(t, contenido.ID().IsZero(), "el objeto existe igual")

	assert.True(t, materialBase(t).componer(t).IsComplete())
}

// ── Componer no escribe nada ni muta nada ──────────────────────────────────

func TestContent_ComponerNoMutaLoQueRecibe(t *testing.T) {
	material := materialBase(t)
	contenido := material.componer(t)

	// El agregado es inmutable: lo que devuelve es una copia, así que tocarla no
	// puede cambiar la identidad ya calculada.
	antes := contenido.ID()
	devueltos := contenido.Steps()
	devueltos[0] = stepSupply(t)

	assert.True(t, antes.Equals(contenido.ID()))
	assert.Equal(t, "01-test", contenido.Steps()[0].StepID())
}

func TestContent_AccesoresDevuelvenLoCompuesto(t *testing.T) {
	contenido := materialBase(t).componer(t)

	assert.Equal(t, sujetoBase, contenido.Subject().String())
	assert.Equal(t, operacionBase, contenido.Operation().String())
	assert.Equal(t, destinoBase, contenido.Destination().String())
	assert.Equal(t, huella(t, fingerprint.Version, "33"), contenido.Source().Project())
	assert.Equal(t, 2, contenido.Format().SchemaVersion())
	assert.Len(t, contenido.Steps(), 2)
}

// ── Vectores congelados ────────────────────────────────────────────────────

func TestContentID_VectoresCongelados(t *testing.T) {
	// La autoridad es este test; si `SPEC-CONTENT-v1.md` §7 y estos literales
	// discrepan, discrepan los dos.
	base := materialBase(t).componer(t)

	assert.Equal(t, contenidoCanonicoEsperado, base.Canonical(),
		"la forma canónica es normativa: un tercero tiene que reproducirla bit a bit")
	assert.Equal(t, vectorContentIDBase, base.ID().String())

	prod := materialBase(t)
	prod.destination = "prod"
	assert.Equal(t, vectorContentIDProd, prod.componer(t).ID().String())

	primero, err := deployment.DeploymentIDOf(base.ID(), deployment.DeploymentID{})
	require.NoError(t, err)
	assert.Equal(t, vectorDeploymentIDRaiz, primero.String())

	segundo, err := deployment.DeploymentIDOf(base.ID(), primero)
	require.NoError(t, err)
	assert.Equal(t, vectorDeploymentIDSegundo, segundo.String())
}
