package command

type ExecutionVariableMap map[string]Variable

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

func (vs ExecutionVariableMap) Add(variable Variable) {
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

func (vs ExecutionVariableMap) ToStringMap() map[string]string {
	m := make(map[string]string, len(vs))
	for k, v := range vs {
		m[k] = v.Value()
	}
	return m
}
