package command

import (
	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
)

type ExecutionVariableMap map[string]Variable

var _ domNotify.Vocabulary = (*ExecutionVariableMap)(nil)

func NewExecutionVariableMap() *ExecutionVariableMap {
	return &ExecutionVariableMap{}
}

func (vs *ExecutionVariableMap) Clone() ExecutionVariableMap {
	clone := make(ExecutionVariableMap, len(*vs))
	for k, v := range *vs {
		clone[k] = v
	}
	return clone
}

func (vs ExecutionVariableMap) ToSlice() []Variable {
	slice := make([]Variable, 0, len(vs))
	for _, variable := range vs {
		slice = append(slice, variable)
	}
	return slice
}

// Add inserta la variable si no existe, o si su origen tiene precedencia mayor
// o IGUAL que la de la ya presente (spec 12 §5.2).
//
// Aquí es donde `ExecutionVariableMap` deja de ser un mapa con métodos y pasa a
// ser lo que siempre fue: el agregado que custodia la resolución de variables,
// con su invariante propia. Antes sobrescribía a ciegas, así que la respuesta a
// «¿qué valor gana?» solo se obtenía leyendo el orden de cuatro handlers en
// `internal/interfaces/cli/factory.go`.
//
// La IGUALDAD importa y no es un descuido: dos comandos del mismo step que
// producen la misma variable deben poder actualizarla —el segundo gana—, y lo
// mismo vale para el paso siguiente que vuelve a leer el almacén. Es el
// comportamiento de hoy y hay que conservarlo.
func (vs ExecutionVariableMap) Add(variable Variable) {
	if current, exists := vs[variable.Name()]; exists && variable.Origin() < current.Origin() {
		return
	}
	vs[variable.Name()] = variable
}

func (vs ExecutionVariableMap) Remove(name string) {
	delete(vs, name)
}

func (vs ExecutionVariableMap) AddAll(variables []Variable) {
	for _, variable := range variables {
		vs.Add(variable)
	}
}

func (vs ExecutionVariableMap) AddAllMap(variables ExecutionVariableMap) {
	for _, variable := range variables {
		vs.Add(variable)
	}
}

// Equals compara `Variable` por igualdad de struct, así que desde la spec 12
// dos mapas con los mismos pares (nombre, valor) llegados por caminos distintos
// son DISTINTOS. No lo llama nadie —la comparación que decide re-ejecutar es la
// de huellas, y ésa no mira el origen (`SPEC-VARIABLES-v1.md` §4)—, y queda
// dicho para que el primero que lo use sepa qué está comparando.
func (vs ExecutionVariableMap) Equals(other ExecutionVariableMap) bool {
	if len(vs) != len(other) {
		return false
	}
	for k, v := range vs {
		if other[k] != v {
			return false
		}
	}
	return true
}

func (vs ExecutionVariableMap) Filter(filter func(Variable) bool) *ExecutionVariableMap {
	filtered := NewExecutionVariableMap()
	for _, v := range vs {
		if filter(v) {
			filtered.Add(v)
		}
	}
	return filtered
}

func (vs ExecutionVariableMap) Get(key string) (Variable, bool) {
	outputVar, exists := vs[key]
	return outputVar, exists
}

// Values implementa `notify.Vocabulary`: los pares (nombre, valor) que el
// proceso conoce en este instante.
//
// Existe para la REDACCIÓN del stream de logs (spec 20 §5.3), y es el mismo dato
// que `ToStringMap` con otro nombre porque el puerto lo pide así. Que sea este
// agregado quien lo implemente no es casualidad: `Add` es el único sitio por el
// que un valor entra al proceso, así que es el único que puede responder «qué
// valores hay» sin que nadie se olvide de registrar el suyo.
func (vs ExecutionVariableMap) Values() map[string]string {
	return vs.ToStringMap()
}

func (vs ExecutionVariableMap) ToStringMap() map[string]string {
	m := make(map[string]string, len(vs))
	for k, v := range vs {
		m[k] = v.Value()
	}
	return m
}
