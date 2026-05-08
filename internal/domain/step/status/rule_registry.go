package status

import (
	"fmt"
	"sync"
)

type RuleRegistry struct {
	rules map[string]Rule
	mu    sync.RWMutex
}

func NewRuleRegistry() *RuleRegistry {
	return &RuleRegistry{
		rules: make(map[string]Rule),
	}
}

func (r *RuleRegistry) Register(rule Rule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[rule.Name()] = rule
}

func (r *RuleRegistry) Get(name string) (Rule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rule, ok := r.rules[name]
	if !ok {
		return nil, fmt.Errorf("rule %q not registered", name)
	}
	return rule, nil
}
