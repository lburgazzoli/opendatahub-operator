// Package bdd provides Behavior-Driven Development testing utilities for Kubernetes resources.
package bdd

import (
	"bytes"
	"fmt"
	"maps"
	"sync"
	"text/template"
	"time"

	"github.com/onsi/gomega/gstruct"
	"github.com/rs/xid"
)

var (
	specialVariables = map[string]interface{}{
		"any":    "<any>",          // Placeholder for any value in assertions
		"ignore": gstruct.Ignore(), // Ignores the field in assertions
		"empty":  "",               // Empty string marker
		"null":   nil,              // Null value marker
	}

	commonTemplateFuncs = template.FuncMap{
		"timestamp": func() string {
			return time.Now().Format(time.RFC3339)
		},
		"uuid": func() string {
			return xid.New().String()
		},
	}
)

type VariableManager struct {
	mu        sync.RWMutex
	variables map[string]interface{}
}

func NewVariableManager() *VariableManager {
	return &VariableManager{
		variables: make(map[string]interface{}),
	}
}

func (vm *VariableManager) SetAll(vars map[string]interface{}) {
	for k, v := range vars {
		vm.Set(k, v)
	}
}

func (vm *VariableManager) Set(name string, value interface{}) {
	if name == "" {
		return
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.variables[name] = value
}

func (vm *VariableManager) Get(name string) (interface{}, bool) {
	if name == "" {
		return nil, false
	}

	vm.mu.RLock()
	defer vm.mu.RUnlock()

	if value, exists := vm.variables[name]; exists {
		return vm.unwrap(value), true
	}
	if value, exists := specialVariables[name]; exists {
		return vm.unwrap(value), true
	}

	return nil, false
}

func (vm *VariableManager) Remove(name string) {
	if name == "" {
		return
	}

	vm.mu.Lock()
	defer vm.mu.Unlock()

	delete(vm.variables, name)
}

func (vm *VariableManager) Has(name string) bool {
	if name == "" {
		return false
	}

	vm.mu.RLock()
	defer vm.mu.RUnlock()

	// Check user variables first
	if _, exists := vm.variables[name]; exists {
		return true
	}

	// Check special variables
	if _, exists := specialVariables[name]; exists {
		return true
	}

	return false
}

func (vm *VariableManager) Clear() {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.variables = make(map[string]interface{})
}

func (vm *VariableManager) Copy() *VariableManager {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return &VariableManager{
		variables: maps.Clone(vm.variables),
	}
}

func (vm *VariableManager) Size() int {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return len(vm.variables)
}

func (vm *VariableManager) GetAll() map[string]interface{} {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	result := make(map[string]interface{})

	// Add special variables first
	for k, v := range specialVariables {
		result[k] = v
	}

	// Add user variables (they can override special variables)
	for k, v := range vm.variables {
		result[k] = vm.unwrap(v)
	}

	return result
}

func (vm *VariableManager) Interpolate(tmpl string) (string, error) {
	if tmpl == "" {
		return "", nil
	}

	t, err := template.New("bdd").
		Option("missingkey=error").
		Funcs(commonTemplateFuncs).
		Parse(tmpl)

	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	// Build template data map with unwrapped values
	allVars := make(map[string]interface{})

	// Add special variables first
	for k, v := range specialVariables {
		allVars[k] = vm.unwrap(v)
	}

	// Add user variables (they take precedence)
	vm.mu.RLock()
	for k, v := range vm.variables {
		allVars[k] = vm.unwrap(v)
	}
	vm.mu.RUnlock()

	// Execute template
	var buf bytes.Buffer
	if err := t.Execute(&buf, allVars); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

func (vm *VariableManager) MustInterpolate(tmpl string) string {
	result, err := vm.Interpolate(tmpl)
	if err != nil {
		panic(err)
	}

	return result
}

// unwrap evaluates a value if it's a function, otherwise returns the value as-is.
// This allows for dynamic variable evaluation where functions are called to get current values.
func (vm *VariableManager) unwrap(in any) any {
	if f, ok := in.(func() any); ok {
		return f()
	}

	return in
}
