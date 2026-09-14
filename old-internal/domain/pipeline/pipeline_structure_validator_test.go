package pipeline_test

// El invariante estructural del pipelinecode (spec 04 §5.1).
//
// Cada caso de esta tabla es un pipelinecode que HOY —antes de la spec 04— se
// ejecutaba con exit code 0 y un step de menos, o con los steps reordenados. La
// diferencia observable no es que falle: es que el mensaje nombra el directorio
// culpable.

import (
	"context"
	"fmt"
	"testing"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	domPipeline "github.com/jairoprogramador/vex-engine/old-internal/domain/pipeline"
	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ctxDePrueba y codigoDePrueba son el contexto que la spec 13 añadió a la firma
// del validador —ruta del pipelinecode— y que la 14 amplió con el ambiente.
// Ninguna de las dos reglas de la spec 04 los mira: los usan la del ámbito, que
// habla de un ARCHIVO del step, y las dos de la gramática de variables, que
// hablan de `variables/<ambiente>/<paso>.yaml`.
func ctxDePrueba() *context.Context {
	c := context.Background()
	return &c
}

func codigoDePrueba() domPipeline.Pipelinecode {
	return domPipeline.Pipelinecode{LocalPath: "/pipelinecode", Environment: "sand"}
}

// configsSinArchivo es el repositorio de `config.yaml` para los casos que no
// hablan de ámbitos: ningún step declara nada, que es legítimo (§5.3).
type configsSinArchivo struct{}

var _ domStep.StepConfigRepository = configsSinArchivo{}

func (configsSinArchivo) Get(*context.Context, string, string) (domStep.StepConfig, error) {
	return domStep.NoStepConfig(), nil
}

func TestPipelineStructureValidator_QueEstructuraSeAcepta(t *testing.T) {
	validador := domPipeline.NewPipelineStructureValidator(
		configsSinArchivo{}, sinManifiesto{}, declaraciones(nil), comandos(nil))

	casos := []struct {
		nombre    string
		entradas  []string
		valido    bool
		enElError []string
		nota      string
	}{
		{
			nombre:   "cuatro steps de dos dígitos",
			entradas: []string{"01-test", "02-supply", "03-package", "04-deploy"},
			valido:   true,
			nota:     "la forma de los tres templates reales",
		},
		{
			nombre:   "doce steps de dos dígitos",
			entradas: []string{"01-a", "02-b", "03-c", "04-d", "05-e", "06-f", "07-g", "08-h", "09-i", "10-j", "11-k", "12-l"},
			valido:   true,
			nota:     "pasar de nueve steps no rompe nada",
		},
		{
			nombre:   "sin steps",
			entradas: nil,
			valido:   true,
			nota:     "un pipelinecode sin steps no es un error de estructura; el paso pedido no existirá y de eso habla el handler",
		},
		{
			nombre:    "prefijo de un dígito",
			entradas:  []string{"2-supply"},
			valido:    false,
			enElError: []string{"steps/2-supply"},
			nota:      "HOY pasa en silencio: el motor busca steps/02-supply/commands.yaml, no lo encuentra y no ejecuta nada",
		},
		{
			nombre:    "un solo directorio de un dígito entre otros correctos",
			entradas:  []string{"01-test", "2-supply", "03-package"},
			valido:    false,
			enElError: []string{"steps/2-supply"},
			nota:      "HOY se ejecuta en el orden 1,3,2",
		},
		{
			nombre:    "prefijo de tres dígitos",
			entradas:  []string{"002-supply"},
			valido:    false,
			enElError: []string{"steps/002-supply"},
			nota:      "HOY colapsa en la misma clave de estado que 02-supply",
		},
		{
			nombre:    "separador guion bajo",
			entradas:  []string{"01-test", "4_test"},
			valido:    false,
			enElError: []string{"steps/4_test"},
			nota:      "HOY desaparece en silencio",
		},
		{
			nombre:    "sin prefijo numérico",
			entradas:  []string{"supply"},
			valido:    false,
			enElError: []string{"steps/supply"},
		},
		{
			nombre:    "el número detrás del nombre",
			entradas:  []string{"package-02"},
			valido:    false,
			enElError: []string{"steps/package-02"},
		},
		{
			nombre:    "prefijo vacío",
			entradas:  []string{"-deploy"},
			valido:    false,
			enElError: []string{"steps/-deploy"},
		},
		{
			nombre:    "nombre vacío",
			entradas:  []string{"02-"},
			valido:    false,
			enElError: []string{"steps/02-"},
		},
		{
			nombre:    "dos steps con el mismo orden",
			entradas:  []string{"01-test", "02-a", "02-b"},
			valido:    false,
			enElError: []string{"steps/02-a", "steps/02-b", "orden 02"},
			nota:      "HOY el orden de ejecución entre los dos es arbitrario y comparten clave de estado",
		},
		{
			nombre:    "varios problemas se reportan de una vez",
			entradas:  []string{"2-supply", "4_test", "03-a", "03-b"},
			valido:    false,
			enElError: []string{"steps/2-supply", "steps/4_test", "steps/03-a", "steps/03-b"},
			nota:      "la validación no corta en el primer fallo (§5.3')",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			err := validador.Validate(ctxDePrueba(), codigoDePrueba(), entradas(caso.entradas...))

			if caso.valido {
				require.NoError(t, err, caso.nota)
				return
			}
			require.Error(t, err, caso.nota)
			for _, fragmento := range caso.enElError {
				assert.Contains(t, err.Error(), fragmento,
					"el error tiene que nombrar al culpable. %s", caso.nota)
			}
		})
	}
}

func TestPipelineStructureValidator_ReglasComponibles(t *testing.T) {
	// OCP (§5.2'): las specs 05 y 15 añaden reglas sin tocar el repositorio ni el
	// handler. Este test fija que el validador es una composición y no una cascada
	// de `if`.
	t.Run("un validador sin reglas acepta cualquier cosa", func(t *testing.T) {
		require.NoError(t,
			domPipeline.NewStepsStructureValidator().Validate(
				ctxDePrueba(), codigoDePrueba(), entradas("4_test", "02-a", "02-b")))
	})

	t.Run("cada regla se puede verificar por separado", func(t *testing.T) {
		soloFormato := domPipeline.NewStepsStructureValidator(domPipeline.NewStepEntryFormatRule())
		soloOrden := domPipeline.NewStepsStructureValidator(domPipeline.NewUniqueStepOrderRule())

		assert.Error(t, soloFormato.Validate(ctxDePrueba(), codigoDePrueba(), entradas("4_test")))
		assert.NoError(t, soloOrden.Validate(ctxDePrueba(), codigoDePrueba(), entradas("4_test")),
			"un directorio ilegible no es un orden duplicado: de él habla la otra regla")

		assert.NoError(t, soloFormato.Validate(ctxDePrueba(), codigoDePrueba(), entradas("02-a", "02-b")))
		assert.Error(t, soloOrden.Validate(ctxDePrueba(), codigoDePrueba(), entradas("02-a", "02-b")))
	})

	t.Run("una regla añadida se evalúa", func(t *testing.T) {
		validador := domPipeline.NewStepsStructureValidator(reglaQueSiempreFalla{})

		err := validador.Validate(ctxDePrueba(), codigoDePrueba(), entradas("01-test"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "regla_de_prueba", "el error identifica la regla que falló")
	})
}

// ── La regla de la configuración del step (specs 13 y 15) ───────────────────

// Lo que la regla exige NO es que el `config.yaml` exista: es que el que exista
// diga algo del vocabulario cerrado. La diferencia es la mitad de §5.3 que es
// fácil de perder, porque las dos situaciones se parecen desde fuera.
func TestStepConfigRule_QueSeExigeDeLoDeclarado(t *testing.T) {
	casos := []struct {
		nombre    string
		declarado map[string]string // directorio → contenido de `scope:`
		valido    bool
		enElError []string
		nota      string
	}{
		{
			nombre:    "ningún step declara nada",
			declarado: nil,
			valido:    true,
			nota:      "sin config.yaml el step se ejecuta siempre y no persiste: legítimo (§5.3)",
		},
		{
			nombre:    "los dos ámbitos del vocabulario",
			declarado: map[string]string{"01-acr": "project", "02-provision": "environment"},
			valido:    true,
		},
		{
			nombre:    "un ámbito inventado",
			declarado: map[string]string{"02-supply": "shared"},
			valido:    false,
			enElError: []string{"steps/02-supply/config.yaml", "shared"},
			nota:      "un ámbito inventado no tendría dónde persistirse",
		},
		{
			nombre:    "config.yaml presente sin scope",
			declarado: map[string]string{"02-supply": ""},
			valido:    false,
			enElError: []string{"steps/02-supply/config.yaml"},
			nota:      "presente es una intención de declarar: tratarlo como ausente sería adivinar cuál",
		},
		{
			nombre:    "sólo uno de varios está mal",
			declarado: map[string]string{"01-test": "environment", "02-supply": "Project"},
			valido:    false,
			enElError: []string{"steps/02-supply/config.yaml", "Project"},
			nota:      "el vocabulario es sensible a mayúsculas",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			nombres := make([]string, 0, len(caso.declarado))
			for dir := range caso.declarado {
				nombres = append(nombres, dir)
			}
			if len(nombres) == 0 {
				nombres = []string{"01-test", "02-supply"}
			}

			validador := domPipeline.NewStepsStructureValidator(
				domPipeline.NewStepConfigRule(configsDeclarados(caso.declarado)))

			err := validador.Validate(ctxDePrueba(), codigoDePrueba(), entradas(nombres...))

			if caso.valido {
				require.NoError(t, err, caso.nota)
				return
			}
			require.Error(t, err, caso.nota)
			for _, fragmento := range caso.enElError {
				assert.Contains(t, err.Error(), fragmento,
					"el error tiene que nombrar al culpable. %s", caso.nota)
			}
		})
	}
}

// configsDeclarados imita al repositorio real: valida el vocabulario al
// traducir, y nombra el archivo en su error.
type configsDeclarados map[string]string

var _ domStep.StepConfigRepository = configsDeclarados(nil)

func (c configsDeclarados) Get(_ *context.Context, _, step string) (domStep.StepConfig, error) {
	declarado, presente := c[step]
	if !presente {
		return domStep.NoStepConfig(), nil
	}
	scope, err := domStep.NewScope(declarado)
	if err != nil {
		return domStep.StepConfig{}, fmt.Errorf("'steps/%s/config.yaml' %w", step, err)
	}
	return domStep.NewStepConfig(scope, domStep.EmptyRuleSet())
}

type reglaQueSiempreFalla struct{}

var _ domPipeline.StepStructureRule = (*reglaQueSiempreFalla)(nil)

func (reglaQueSiempreFalla) Name() string { return "regla_de_prueba" }

func (reglaQueSiempreFalla) IsSatisfiedBy(*context.Context, domPipeline.Pipelinecode, []domPipeline.StepEntry) error {
	return assert.AnError
}

// ── Las dos reglas de la gramática de variables (spec 14) ───────────────────

// El manifiesto es lo que permite añadir gramática sin romper a nadie en
// silencio: sin `vexpipeline.yaml` el pipelinecode está en la versión 1, se
// ejecuta igual que ayer y `resolve` está PROHIBIDO —con un mensaje que nombra la
// versión que haría falta, en vez de un fallo a mitad del despliegue—.
func TestDeclaredSourceVersionRule_QueVersionExigeResolve(t *testing.T) {
	conFuente := declaraciones(map[string][]domStep.VariableDeclaration{
		"package": {declaracionDeOutput(t, "acr", "02-supply", "acr_name")},
	})
	soloLiterales := declaraciones(map[string][]domStep.VariableDeclaration{
		"package": {declaracionLiteral(t, "instance_count", "3")},
	})

	casos := []struct {
		nombre     string
		manifiesto domPipeline.ManifestRepository
		declaradas declaraciones
		valido     bool
		enElError  []string
		nota       string
	}{
		{
			nombre:     "sin manifiesto y sólo literales",
			manifiesto: sinManifiesto{},
			declaradas: soloLiterales,
			valido:     true,
			nota:       "es todo el pipelinecode escrito hasta hoy: se ejecuta igual",
		},
		{
			nombre:     "sin manifiesto y con resolve",
			manifiesto: sinManifiesto{},
			declaradas: conFuente,
			valido:     false,
			enElError:  []string{"acr", "schema_version: 2", "vexpipeline.yaml"},
			nota:       "la ausencia del manifiesto es la versión 1, y en la 1 `resolve` no existe",
		},
		{
			nombre:     "schema_version 1 explícito y con resolve",
			manifiesto: manifiestoDeVersion(t, domPipeline.SchemaVersion1),
			declaradas: conFuente,
			valido:     false,
			enElError:  []string{"schema_version: 2"},
			nota:       "declarar la versión vieja no habilita la gramática nueva",
		},
		{
			nombre:     "schema_version 2 y con resolve",
			manifiesto: manifiestoDeVersion(t, domPipeline.SchemaVersion2),
			declaradas: conFuente,
			valido:     true,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			validador := domPipeline.NewStepsStructureValidator(
				domPipeline.NewDeclaredSourceVersionRule(caso.manifiesto, caso.declaradas))

			err := validador.Validate(ctxDePrueba(), codigoDePrueba(),
				entradas("01-test", "02-supply", "03-package"))

			if caso.valido {
				require.NoError(t, err, caso.nota)
				return
			}
			require.Error(t, err, caso.nota)
			for _, fragmento := range caso.enElError {
				assert.Contains(t, err.Error(), fragmento, caso.nota)
			}
		})
	}
}

// Las dos validaciones de §5.4, que sólo son posibles porque `from` hace
// EXPLÍCITO el grafo de dependencias entre steps. Las dos mueven un fallo de
// ejecución —después de que `test` y `supply` ya tuvieron efectos reales— a un
// fallo de carga.
func TestVariableGraphRule_QueSeExigeDeUnStepOutput(t *testing.T) {
	casos := []struct {
		nombre     string
		declaradas map[string][]domStep.VariableDeclaration
		valido     bool
		enElError  []string
		nota       string
	}{
		{
			nombre: "apunta a un step anterior que declara ese output",
			declaradas: map[string][]domStep.VariableDeclaration{
				"package": {declaracionDeOutput(t, "acr", "02-supply", "acr_name")},
			},
			valido: true,
		},
		{
			nombre: "apunta a un step POSTERIOR",
			declaradas: map[string][]domStep.VariableDeclaration{
				"test": {declaracionDeOutput(t, "acr", "02-supply", "acr_name")},
			},
			valido:    false,
			enElError: []string{"01-test", "from: 02-supply", "no se ejecuta antes"},
			nota:      "HOY falla a mitad del despliegue con «variable no existe»",
		},
		{
			nombre: "apunta a sí mismo",
			declaradas: map[string][]domStep.VariableDeclaration{
				"supply": {declaracionDeOutput(t, "acr", "02-supply", "acr_name")},
			},
			valido:    false,
			enElError: []string{"no se ejecuta antes"},
			nota:      "la declaración se satisface al cargar el step, antes de su primer comando",
		},
		{
			nombre: "apunta a un step que no existe",
			declaradas: map[string][]domStep.VariableDeclaration{
				"package": {declaracionDeOutput(t, "acr", "07-inventado", "acr_name")},
			},
			valido:    false,
			enElError: []string{"steps/07-inventado"},
		},
		{
			nombre: "apunta a un key que nadie declara",
			declaradas: map[string][]domStep.VariableDeclaration{
				"package": {declaracionDeOutput(t, "acr", "02-supply", "acr_nombre")},
			},
			valido:    false,
			enElError: []string{"key: acr_nombre", "02-supply"},
			nota:      "HOY lo resolvería otro step que casualmente declaró el mismo nombre",
		},
		{
			nombre: "un literal no dice nada del grafo",
			declaradas: map[string][]domStep.VariableDeclaration{
				"package": {declaracionLiteral(t, "instance_count", "3")},
			},
			valido: true,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			validador := domPipeline.NewStepsStructureValidator(
				domPipeline.NewVariableGraphRule(
					declaraciones(caso.declaradas),
					comandos(map[string][]string{"02-supply": {"acr_name"}})))

			err := validador.Validate(ctxDePrueba(), codigoDePrueba(),
				entradas("01-test", "02-supply", "03-package"))

			if caso.valido {
				require.NoError(t, err, caso.nota)
				return
			}
			require.Error(t, err, caso.nota)
			for _, fragmento := range caso.enElError {
				assert.Contains(t, err.Error(), fragmento, caso.nota)
			}
		})
	}
}

// ── Dobles de la gramática ──────────────────────────────────────────────────

type sinManifiesto struct{}

var _ domPipeline.ManifestRepository = sinManifiesto{}

func (sinManifiesto) Get(*context.Context, string) (domPipeline.Manifest, error) {
	return domPipeline.NoManifest(), nil
}

type manifiestoFijo struct{ manifest domPipeline.Manifest }

var _ domPipeline.ManifestRepository = manifiestoFijo{}

func (m manifiestoFijo) Get(*context.Context, string) (domPipeline.Manifest, error) {
	return m.manifest, nil
}

func manifiestoDeVersion(t *testing.T, version int) domPipeline.ManifestRepository {
	t.Helper()
	manifest, err := domPipeline.NewManifest(version)
	require.NoError(t, err)
	return manifiestoFijo{manifest: manifest}
}

// declaraciones es `variables/<ambiente>/<paso>.yaml`, indexado por el nombre del
// step SIN su prefijo de orden, que es como se llama el archivo.
type declaraciones map[string][]domStep.VariableDeclaration

var _ domStep.VarsPipelineRepository = declaraciones(nil)

func (d declaraciones) Get(_ *context.Context, _, _, step string) ([]domStep.VariableDeclaration, error) {
	return d[step], nil
}

// comandos es `steps/NN-x/commands.yaml` reducido a lo que la regla del grafo
// mira: qué nombres declara cada step en sus `outputs`.
type comandos map[string][]string

var _ domStep.PipelineCommandRepository = comandos(nil)

func (c comandos) Get(_ *context.Context, _, step string) ([]command.Command, error) {
	outputs := make([]command.CommandOutput, 0, len(c[step]))
	for _, name := range c[step] {
		output, err := command.NewCommandOutput(name, "probe = (.+)")
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output)
	}
	cmd, err := command.NewCommand("provision", "echo hola", command.WithOutputs(outputs))
	if err != nil {
		return nil, err
	}
	return []command.Command{cmd}, nil
}

func declaracionLiteral(t *testing.T, name, value string) domStep.VariableDeclaration {
	t.Helper()
	declaration, err := domStep.NewLiteralDeclaration(name, value)
	require.NoError(t, err)
	return declaration
}

func declaracionDeOutput(t *testing.T, name, from, key string) domStep.VariableDeclaration {
	t.Helper()
	declaration, err := domStep.NewStepOutputDeclaration(name, from, key)
	require.NoError(t, err)
	return declaration
}
