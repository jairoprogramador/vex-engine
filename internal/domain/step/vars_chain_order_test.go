package step_test

// El canario de la spec 12 §5.2': el orden de los handlers de variables dejó de
// ser normativo.
//
// Hasta la 12 la respuesta a «¿qué valor gana?» solo se podía obtener leyendo
// `chainStepHandlers` en `internal/interfaces/cli/factory.go` y siguiendo el
// orden de cuatro handlers: una regla de negocio viviendo en la configuración de
// la infraestructura. Ahora vive en `command.ExecutionVariableMap.Add`, sobre el
// enum ordenado `command.Origin`, y estos casos lo miden con los handlers 01 y
// 02 REALES armados en las dos permutaciones posibles.
//
// Eran tres handlers y seis permutaciones hasta la spec 13, que colapsa los dos
// del almacén en uno solo. El orden ENTRE ÁMBITOS —proyecto primero, ambiente
// después— pasa a ser interno a ese handler y se mide igual, con un nombre que
// los dos aportan: `en_ambos`.
//
// Se prueba aquí y no en el harness de integración porque construir la cadena en
// otro orden desde allí exigiría un punto de extensión en `BuildRunCommand` que
// solo usarían los tests: el cableado de producción diría una cosa y el test
// mediría otra.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domState "github.com/jairoprogramador/vex-engine/internal/domain/state"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

func TestVarsChain_ElOrdenDeLosHandlersYaNoDecideQuienGana(t *testing.T) {
	// El mismo nombre aportado por las CUATRO fuentes a la vez. Es el choque que
	// la precedencia existe para resolver, y el único caso donde el orden podía
	// notarse.
	const nombre = "acr_name"

	esperado := map[string]string{
		// Runtime gana siempre: lo produjo un comando de un step anterior en ESTA
		// ejecución.
		nombre: "de-runtime",
		// Y cada fuente conserva lo suyo cuando nadie compite por el nombre.
		"solo_declarada": "literal",
		"solo_proyecto":  "del-ambito-proyecto",
		"solo_ambiente":  "del-ambito-ambiente",
		"solo_inyectada": "demo-app",
		// Los DOS ámbitos del almacén aportan este nombre con el mismo origen
		// (`OriginState`), así que gana el que llega después — y el handler carga
		// proyecto primero, ambiente después (spec 13 §5.4). Es la regla de
		// igualdad de `Add`, no el cableado.
		"en_ambos": "del-ambito-ambiente",
	}

	for _, orden := range permutacionesDeCadena() {
		t.Run(orden.nombre, func(t *testing.T) {
			executionContext := contextoDePrueba(t)

			// Lo que ya está en el mapa cuando la cadena del step arranca: las
			// variables que inyectó el handler 07 del pipeline y lo que extrajo el
			// handler 05 de un step anterior.
			executionContext.AddAccumulatedVar(
				varDePrueba(t, nombre, "de-runtime", command.OriginRuntime))
			executionContext.AddAccumulatedVar(
				varDePrueba(t, "solo_inyectada", "demo-app", command.OriginInjected))
			executionContext.AddAccumulatedVar(
				varDePrueba(t, nombre, "inyectada-homonima", command.OriginInjected))

			registros := registrosConVariables(t)
			cadena := orden.armar(
				domStep.NewVarsStoreHandler(registros),
				domStep.NewVarsHandler(
					varsDeclaradasDePrueba{}, domStep.NewDeclarationResolvers(registros)),
			)

			request := domStep.NewStepRequestHandler(executionContext, "supply")
			require.NoError(t, cadena.Handle(request.Ctx(), request))

			assert.Equal(t, esperado, request.AccumulatedVars().ToStringMap())
		})
	}
}

// ── Permutaciones ───────────────────────────────────────────────────────────

type ordenDeCadena struct {
	nombre string
	// armar recibe los dos handlers en el orden almacén, declaradas y los
	// encadena en el suyo.
	armar func(handlers ...domStep.StepHandler) domStep.StepHandler
}

// permutacionesDeCadena devuelve las dos formas de encadenar los dos handlers.
// La primera es la de producción —almacén primero, declaradas después— y la
// segunda es la que la spec 12 §5.3 mandaba y su §10 retira.
//
// Lo que estos casos miden es QUIÉN GANA, y ahí las dos coinciden. Lo que NO son
// es una licencia para reordenar: el handler de las declaradas resuelve
// interpolaciones contra el mapa acumulado, así que cargarlo antes que el
// almacén le quita material de vista. Eso lo fija
// `TestRunCommand_UnLiteralPuedeInterpolarElRegistroDelPropioStep`.
func permutacionesDeCadena() []ordenDeCadena {
	nombres := []string{"declaradas", "almacen"}
	indices := [][2]int{
		{1, 0}, // el orden de producción
		{0, 1}, // el que la spec 12 §5.3 pedía, retirado por su §10
	}

	ordenes := make([]ordenDeCadena, 0, len(indices))
	for _, idx := range indices {
		nombre := nombres[idx[0]] + " → " + nombres[idx[1]]
		ordenes = append(ordenes, ordenDeCadena{
			nombre: nombre,
			armar: func(handlers ...domStep.StepHandler) domStep.StepHandler {
				encadenados := []domStep.StepHandler{handlers[idx[0]], handlers[idx[1]]}
				for i := 0; i < len(encadenados)-1; i++ {
					encadenados[i].SetNext(encadenados[i+1])
				}
				return encadenados[0]
			},
		})
	}
	return ordenes
}

// ── Dobles ──────────────────────────────────────────────────────────────────

func varDePrueba(t *testing.T, nombre, valor string, origen command.Origin) command.Variable {
	t.Helper()
	v, err := command.NewVariable(nombre, valor, origen)
	require.NoError(t, err)
	return v
}

// registrosDePrueba devuelve, por ámbito, lo que una corrida anterior dejó.
//
// Las variables salen con `OriginState` porque es lo que hace el adaptador al
// leer del disco (`file_step_record_dto.go`): los handlers 01 y 02 no fabrican
// la marca, la reciben ya puesta.
type registrosDePrueba struct{ t *testing.T }

var _ domState.Records = (*registrosDePrueba)(nil)

func registrosConVariables(t *testing.T) domState.Records {
	return &registrosDePrueba{t: t}
}

func (r *registrosDePrueba) Last(_ *context.Context, key domState.Key) (domState.StepRecord, bool, error) {
	nombrePropio := "solo_ambiente"
	valor := "del-ambito-ambiente"
	if key.Scope().IsProject() {
		nombrePropio = "solo_proyecto"
		valor = "del-ambito-proyecto"
	}

	return domState.NewUnattributedRecord([]command.Variable{
		varDePrueba(r.t, "acr_name", "almacenada-homonima", command.OriginState),
		varDePrueba(r.t, nombrePropio, valor, command.OriginState),
		// El nombre que los DOS ámbitos aportan: mide el orden interno del
		// handler, que dejó de ser el orden de la cadena con la spec 13.
		varDePrueba(r.t, "en_ambos", valor, command.OriginState),
	}), true, nil
}

func (r *registrosDePrueba) Append(*context.Context, domState.Key, domState.StepRecord) error {
	return nil
}

// varsDeclaradasDePrueba es el `variables/<ambiente>/<paso>.yaml` del
// pipelinecode.
//
// Devuelve DECLARACIONES desde la spec 14, no variables ya resueltas: el
// repositorio real dejó de resolver por su cuenta y el handler 02 es quien
// convierte cada declaración en un valor —los literales con `OriginDeclared`,
// las que nombran una fuente con `OriginResolved`—.
type varsDeclaradasDePrueba struct{}

var _ domStep.VarsPipelineRepository = varsDeclaradasDePrueba{}

func (varsDeclaradasDePrueba) Get(_ *context.Context, _, _, _ string) ([]domStep.VariableDeclaration, error) {
	declarada, err := domStep.NewLiteralDeclaration("acr_name", "literal-homonimo")
	if err != nil {
		return nil, err
	}
	propia, err := domStep.NewLiteralDeclaration("solo_declarada", "literal")
	if err != nil {
		return nil, err
	}
	return []domStep.VariableDeclaration{declarada, propia}, nil
}
