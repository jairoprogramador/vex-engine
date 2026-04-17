package vos

type VariableSet map[string]OutputVar

func NewVariableSetFromMap(varsMap map[string]string) (VariableSet, error) {
	vs := NewVariableSet()
	for name, value := range varsMap {
		v, err := NewOutputVar(name, value, false)
		if err != nil {
			return nil, err
		}
		vs.Add(v)
	}
	return vs, nil
}

func NewVariableSet() VariableSet {
	return make(VariableSet)
}

func (vs VariableSet) Clone() VariableSet {
	clone := make(VariableSet, len(vs))
	for k, v := range vs {
		clone[k] = v
	}
	return clone
}

func (vs VariableSet) Add(outputVar OutputVar) {
	vs[outputVar.Name()] = outputVar
}

func (vs VariableSet) AddAll(other VariableSet) {
	for k, v := range other {
		vs[k] = v
	}
}

func (vs VariableSet) Equals(other VariableSet) bool {
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

func (vs VariableSet) Filter(filter func(OutputVar) bool) VariableSet {
	filtered := NewVariableSet()
	for _, v := range vs {
		if filter(v) {
			filtered.Add(v)
		}
	}
	return filtered
}

func (vs VariableSet) Get(key string) (OutputVar, bool) {
	outputVar, exists := vs[key]
	return outputVar, exists
}

func (vs VariableSet) ToStringMap() map[string]string {
	m := make(map[string]string, len(vs))
	for k, v := range vs {
		m[k] = v.Value()
	}
	return m
}