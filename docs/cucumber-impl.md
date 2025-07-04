# BDD Testing Framework Implementation Notes

## Overview
This document tracks the implementation progress and notes for the BDD testing framework for the OpenDataHub operator. The framework enables writing cucumber-style tests using Gherkin language and godog.

## Architecture Summary
- **Package Location**: `pkg/utils/test/bdd/`
- **Core Pattern**: Composable functions with minimal abstractions
- **Key Technologies**: godog, controller-runtime client, JQ expressions, Gomega matchers
- **Resource Management**: Kubernetes RESTMapper for discovery, unstructured objects for flexibility

## Implementation Progress

### ✅ Design Phase Complete
- [x] Reviewed design document (docs/cucumber.md)
- [x] Understood existing test patterns in codebase
- [x] Identified integration points with current test framework

### ✅ Task 1: Package Structure and Core Types
- [x] Created package structure in `pkg/utils/test/bdd/`
- [x] Implemented Viper-based configuration management
- [x] Created TestContext with variable management
- [x] Added comprehensive test coverage
- [x] Validated all components compile and pass tests

### 🔄 Current Session Focus
Implementing remaining tasks for the BDD framework.

## Key Design Decisions

### 1. Leverage Existing Kubernetes APIs
- Use `meta.RESTMapping` directly instead of custom ResourceInfo struct
- Leverage `k8s.io/client-go/discovery` for resource discovery
- Use `k8s.io/apimachinery/pkg/apis/meta/v1/unstructured` for resource operations

### 2. Compose Functions Rather Than Abstract
- Core CRUD functions: `CreateResource`, `GetResource`, `UpdateResource`, `DeleteResource`
- Helper functions that compose core functions: `GetResourceWithDiscovery`, `GetSingletonResource`
- Step definitions compose these functions rather than creating new abstractions

### 3. Direct Integration with Gomega
- Use Gomega matchers directly instead of custom Assertion types
- Leverage `Eventually` and `Consistently` from Gomega
- Use JQ expressions with custom Gomega matchers

### 4. Variable System with Go Templates
- Use standard Go `text/template` for interpolation
- Support special variables: `{{any}}`, `{{ignore}}`, `{{timestamp}}`
- Dynamic variables evaluated via JQ expressions

## Implementation Strategy

### Phase 1: Core Foundation (Tasks 1-4)
1. Package structure and configuration management
2. Resource discovery with caching
3. Variable management with Go templates
4. Basic resource operations (CRUD)

### Phase 2: JQ Integration (Task 5)
- Integrate JQ library for expression evaluation
- Create Gomega matchers for JQ expressions
- Handle special variables in assertions

### Phase 3: Step Definitions (Tasks 6-10)
- Configuration steps
- Variable management steps
- Resource creation steps
- Resource assertion steps
- Resource modification and deletion steps

### Phase 4: Integration and Testing (Tasks 11-15)
- Test runner setup
- Integration with existing test framework
- Performance optimization
- Documentation and examples

## Technical Notes

### Resource Discovery Pattern
```go
// Use Kubernetes RESTMapping directly
func (r *ResourceResolver) Resolve(resourceRef string) (*meta.RESTMapping, schema.GroupVersionKind, error) {
    // 1. Try direct lookup in cache
    // 2. Use discovery client to find resource
    // 3. Create RESTMapping from discovery info
    // 4. Cache result for future use
}
```

### Scope Validation
```go
func ValidateResourceScope(mapping *meta.RESTMapping, hasNamespace bool) error {
    isNamespaced := mapping.Scope.Name() == meta.RESTScopeNamespace.Name()
    // Validate namespace provided for namespaced resources
    // Validate no namespace for cluster-scoped resources
}
```

### JQ Integration
- Use `github.com/itchyny/gojq` for JQ expression evaluation
- Create custom Gomega matcher: `MatchJQExpression(expr string, expected interface{})`
- Handle special variables before JQ evaluation

### Step Definition Pattern
```go
func initializeSteps(ctx *godog.ScenarioContext, testCtx *TestContext) {
    ctx.Step(`^I create a "([^"]*)" named "([^"]*)" in namespace "([^"]*)" with:$`,
        func(resourceType, name, namespace string, docString *godog.DocString) error {
            // 1. Resolve resource type
            // 2. Validate scope
            // 3. Interpolate variables
            // 4. Parse YAML
            // 5. Create resource
        })
}
```

## Integration Points

### With Existing Test Framework
- Use existing scheme setup from `pkg/utils/test/scheme`
- Leverage existing fake client utilities
- Integrate with existing Gomega patterns

### With ODH Components
- Support DSC and DSCI singleton resources
- Handle component-specific resources
- Integrate with existing component test helpers

## Key Learnings and Findings

### Configuration Management Patterns
- **Viper Isolation**: Using `viper.New()` instead of global instance provides better test isolation
- **Environment Variable Support**: `BDD_` prefix allows CI/CD configuration override
- **Type-Safe Getters**: Wrapper functions prevent runtime type assertion errors

### Project Integration Observations
- **Existing Dependencies**: Project uses testify, can be leveraged for BDD tests
- **Import Structure**: Standard Kubernetes client-go and controller-runtime patterns work well
- **Test Patterns**: Subtests with descriptive names improve readability and debugging

### Variable Management Design
- **Copy-on-Read**: Prevents external modification of test context state
- **Nil Validation**: Early validation prevents runtime panics
- **Clear Error Messages**: Variable not found errors include variable name for debugging

## Open Questions / TODOs

1. **JQ Library Choice**: Confirm `github.com/itchyny/gojq` is appropriate
2. **Error Context**: Ensure error messages include sufficient context
3. **Performance**: Profile discovery operations under load
4. **Extensibility**: Design hooks for custom step definitions
5. **Documentation**: Create comprehensive examples and troubleshooting guide

## Task File Convention
Each implementation task is documented in a separate file following the naming pattern:
- `docs/cucumber-impl-task-N.md` where N is the task number
- Example: `docs/cucumber-impl-task-1.md`, `docs/cucumber-impl-task-2.md`, etc.

## Next Steps
Continue with Task 3: Implement Variable Management (variables.go).