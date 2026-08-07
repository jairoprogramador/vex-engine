package command_test

// La precedencia de variables, que hasta la spec 12 era una propiedad emergente
// del orden de `chainStepHandlers` y ahora es una invariante del agregado.
//
// Estos casos son la parte del §7 de la spec que no necesita el motor entero;
// los que sí lo necesitan —un literal contra un `outputs` real, el canario de
// independencia del orden— están en
// `internal/interfaces/cli/run_command_integration_test.go`.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

func varDe(t *testing.T, nombre, valor string, origen command.Origin) command.Variable {
	t.Helper()
	v, err := command.NewVariable(nombre, valor, origen)
	require.NoError(t, err)
	return v
}

func TestExecutionVariableMap_Add_Precedencia(t *testing.T) {
	casos := []struct {
		nombre   string
		primero  command.Origin
		segundo  command.Origin
		esperado string
		nota     string
	}{
		{
			nombre:   "runtime pisa a lo declarado",
			primero:  command.OriginDeclared,
			segundo:  command.OriginRuntime,
			esperado: "segundo",
			nota:     "EL caso que da nombre a la spec: el literal es un default",
		},
		{
			nombre:   "lo declarado NO pisa a runtime, llegue cuando llegue",
			primero:  command.OriginRuntime,
			segundo:  command.OriginDeclared,
			esperado: "primero",
			nota:     "es lo que hace que el orden de los handlers deje de importar",
		},
		{
			nombre:   "el almacén NO pisa lo inyectado por el motor",
			primero:  command.OriginInjected,
			segundo:  command.OriginState,
			esperado: "primero",
			nota:     "un valor de una corrida anterior no puede pisar un hecho de ésta",
		},
		{
			nombre:   "lo inyectado sí pisa al almacén",
			primero:  command.OriginState,
			segundo:  command.OriginInjected,
			esperado: "segundo",
		},
		{
			nombre:   "el almacén pisa a lo declarado",
			primero:  command.OriginDeclared,
			segundo:  command.OriginState,
			esperado: "segundo",
			nota:     "P3: el literal es el default, lo que ya existe manda",
		},
		{
			nombre:   "runtime pisa a runtime: el segundo comando gana",
			primero:  command.OriginRuntime,
			segundo:  command.OriginRuntime,
			esperado: "segundo",
			nota:     "la IGUALDAD de §5.2: dos comandos del mismo step actualizan",
		},
		{
			nombre:   "el ámbito de ambiente pisa al de proyecto: misma posición",
			primero:  command.OriginState,
			segundo:  command.OriginState,
			esperado: "segundo",
			nota:     "los dos almacenes comparten posición; gana el que carga después",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			vars := command.NewExecutionVariableMap()
			vars.Add(varDe(t, "x", "primero", caso.primero))
			vars.Add(varDe(t, "x", "segundo", caso.segundo))

			resultado, existe := vars.Get("x")
			require.True(t, existe)
			assert.Equal(t, caso.esperado, resultado.Value(), caso.nota)
		})
	}
}

// El orden en que se alimenta el mapa deja de ser semántico: es lo que demuestra
// que la regla dejó de ser emergente (spec 12 §7).
func TestExecutionVariableMap_Add_ElOrdenDeLlegadaNoCambiaElResultado(t *testing.T) {
	origenes := []command.Origin{
		command.OriginDeclared,
		command.OriginState,
		command.OriginInjected,
		command.OriginRuntime,
	}

	// Las 24 permutaciones de los cuatro orígenes: en todas gana runtime.
	var permutar func(restantes []command.Origin, acumulado []command.Origin)
	permutaciones := 0
	permutar = func(restantes []command.Origin, acumulado []command.Origin) {
		if len(restantes) == 0 {
			permutaciones++
			vars := command.NewExecutionVariableMap()
			for _, origen := range acumulado {
				vars.Add(varDe(t, "x", origen.String(), origen))
			}
			resultado, existe := vars.Get("x")
			require.True(t, existe)
			assert.Equal(t, "runtime", resultado.Value(),
				"orden de llegada: %v", acumulado)
			return
		}
		for i, origen := range restantes {
			resto := make([]command.Origin, 0, len(restantes)-1)
			resto = append(resto, restantes[:i]...)
			resto = append(resto, restantes[i+1:]...)
			permutar(resto, append(acumulado, origen))
		}
	}
	permutar(origenes, nil)
	assert.Equal(t, 24, permutaciones)
}

// Una variable que no existe entra siempre, venga de donde venga: la regla es
// «no existe, o precedencia mayor o igual», no «precedencia mayor o igual».
func TestExecutionVariableMap_Add_LoQueNoExisteEntraSiempre(t *testing.T) {
	vars := command.NewExecutionVariableMap()
	vars.Add(varDe(t, "x", "runtime", command.OriginRuntime))
	vars.Add(varDe(t, "y", "literal", command.OriginDeclared))

	literal, existe := vars.Get("y")
	require.True(t, existe, "un literal sin competencia sigue siendo el valor")
	assert.Equal(t, "literal", literal.Value())
}

// AddAll y AddAllMap pasan por Add: la precedencia no se puede saltar entrando
// por otra puerta.
func TestExecutionVariableMap_LasPuertasDeEntradaRespetanLaPrecedencia(t *testing.T) {
	t.Run("AddAll", func(t *testing.T) {
		vars := command.NewExecutionVariableMap()
		vars.Add(varDe(t, "x", "runtime", command.OriginRuntime))
		vars.AddAll([]command.Variable{varDe(t, "x", "literal", command.OriginDeclared)})

		resultado, _ := vars.Get("x")
		assert.Equal(t, "runtime", resultado.Value())
	})

	t.Run("AddAllMap", func(t *testing.T) {
		declaradas := command.NewExecutionVariableMap()
		declaradas.Add(varDe(t, "x", "literal", command.OriginDeclared))

		vars := command.NewExecutionVariableMap()
		vars.Add(varDe(t, "x", "runtime", command.OriginRuntime))
		vars.AddAllMap(*declaradas)

		resultado, _ := vars.Get("x")
		assert.Equal(t, "runtime", resultado.Value())
	})
}

// Remove no consulta precedencia: retirar no es aportar un valor. Es lo que
// hace `StepExecutable` con `step_workdir` al terminar cada step.
func TestExecutionVariableMap_RemoveNoConsultaPrecedencia(t *testing.T) {
	vars := command.NewExecutionVariableMap()
	vars.Add(varDe(t, "step_workdir", "/tmp/uno", command.OriginInjected))
	vars.Remove("step_workdir")

	_, existe := vars.Get("step_workdir")
	assert.False(t, existe)
}
