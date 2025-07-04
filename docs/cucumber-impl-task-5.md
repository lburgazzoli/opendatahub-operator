# Task 5: Integrate JQ-based Assertions with BDD Framework

## Status: 🔄 IN PROGRESS  

## Overview
Integrate existing JQ matcher functionality from `pkg/utils/test/matchers/jq` with the BDD testing framework. Rather than reimplementing JQ capabilities, this task focuses on creating BDD-specific wrappers and helpers that leverage the robust existing JQ infrastructure.

## Discovered Existing JQ Infrastructure

The project already has comprehensive JQ support in `pkg/utils/test/matchers/jq/`:

### ✅ **Available Core Functions:**
- `jq.Match(expression)` - Gomega matcher for JQ boolean expressions
- `jq.Extract(expression)` - Transform function for value extraction  
- `jq.ExtractValue[T](in, expression)` - Type-safe value extraction with generics
- Full support for Kubernetes `unstructured.Unstructured` objects

### ✅ **Existing Features:**
- JQ expression compilation and evaluation using `github.com/itchyny/gojq`
- Comprehensive type conversion (JSON, strings, bytes, readers, Kubernetes objects)
- Proper Gomega integration with clear error messages
- Extensive test coverage with real-world scenarios
- Thread-safe operations and error handling

## Task 5 Revised Scope

Since the core JQ functionality exists, Task 5 will focus on:

## Files to Create/Modify

### `/pkg/utils/test/bdd/assertions.go` (New)
- **BDD-specific wrapper functions** around existing JQ matchers
- **Resource assertion helpers** that combine resource operations with JQ validation
- **Integration with TestContext** for variable interpolation in JQ expressions
- **Convenience functions** for common BDD assertion patterns
- **Error context enhancement** with BDD-specific debugging information

### `/pkg/utils/test/bdd/assertions_test.go` (New)  
- Integration tests combining BDD components (Variables, Resources, JQ assertions)
- Test scenarios showing BDD workflow with JQ assertions
- Performance and usability validation
- Real-world Kubernetes resource validation examples

## Implementation Plan

### 1. BDD Integration Layer

#### JQ Expression with Variable Interpolation
```go
// assertions.go
import (
    "github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
)

// EvaluateJQWithVariables interpolates variables in JQ expressions before evaluation
func EvaluateJQWithVariables(variables *VariableManager, obj interface{}, expression string) (interface{}, error)

// MatchJQWithVariables creates a Gomega matcher with variable interpolation
func MatchJQWithVariables(variables *VariableManager, expression string) types.GomegaMatcher

// ExtractValueWithVariables extracts values with interpolated expressions
func ExtractValueWithVariables[T any](variables *VariableManager, obj interface{}, expression string) (T, error)
```

#### Resource + JQ Assertion Helpers
```go
// AssertResourceMatchesJQ combines resource retrieval with JQ assertion
func AssertResourceMatchesJQ(
    ctx context.Context,
    client client.Client, 
    resolver *ResourceResolver,
    variables *VariableManager,
    resourceType string,
    name string, 
    namespace string,
    jqExpression string,
) error

// AssertSingletonResourceMatchesJQ validates singleton resources with JQ
func AssertSingletonResourceMatchesJQ(
    ctx context.Context,
    client client.Client,
    resolver *ResourceResolver, 
    variables *VariableManager,
    resourceType string,
    jqExpression string,
) error
```

### 2. Convenience Wrappers

#### Common Assertion Patterns
```go
// Convenience functions for frequent BDD patterns
func ResourceExists(resourceType string, name string, namespace string) ResourceAssertionBuilder
func ResourceHasValue(jqExpression string, expectedValue interface{}) ResourceAssertionBuilder
func ResourceMatches(jqExpression string) ResourceAssertionBuilder

// Example usage:
// ResourceExists("pod", "my-pod", "default").
//     AndHasValue(".status.phase", "Running").
//     AndMatches(".spec.containers | length == 1").
//     Assert(ctx, client, resolver, variables)
```

#### Gomega-style BDD Matchers  
```go
// BDD-specific matchers that work with TestContext
func HaveBDDValue(ctx *TestContext, jqExpression string, expected interface{}) types.GomegaMatcher
func MatchBDDExpression(ctx *TestContext, jqExpression string) types.GomegaMatcher
func ExistInCluster(ctx *TestContext, resourceType string, name string, namespace string) types.GomegaMatcher
```

### 3. Integration with Existing BDD Components

#### TestContext Integration
```go  
// Add JQ assertion methods to TestContext for fluent API
func (tc *TestContext) AssertResource(resourceType string, name string, namespace string) *ResourceAssertion
func (tc *TestContext) AssertSingleton(resourceType string) *ResourceAssertion

// ResourceAssertion provides fluent assertion interface
type ResourceAssertion struct {
    ctx        *TestContext
    resourceType string
    name       string
    namespace  string
}

func (ra *ResourceAssertion) Matches(jqExpression string) error
func (ra *ResourceAssertion) HasValue(jqExpression string, expected interface{}) error
func (ra *ResourceAssertion) Exists() error
func (ra *ResourceAssertion) DoesNotExist() error
```

#### Variable Integration
- Support `{{variable}}` placeholders in JQ expressions
- Store JQ evaluation results back into variables
- Template-based assertion patterns

### 4. Enhanced Error Messages

#### BDD-specific Error Context
```go
type BDDAssertionError struct {
    ResourceType string
    Name         string
    Namespace    string
    JQExpression string
    Variables    map[string]interface{}
    Underlying   error
}

func (e *BDDAssertionError) Error() string
func (e *BDDAssertionError) Unwrap() error
```

## Integration Points

### Existing JQ Package
- **Leverage `jq.Match()`** for all boolean JQ expressions
- **Use `jq.ExtractValue[T]()`** for value extraction with type safety  
- **Utilize existing type conversion** for Kubernetes objects
- **Maintain compatibility** with existing JQ infrastructure

### BDD Framework Integration
- **Task 1 (Config)**: Configuration for assertion timeouts and debug settings
- **Task 2 (Discovery)**: Resource type resolution for assertion helpers
- **Task 3 (Variables)**: Variable interpolation in JQ expressions  
- **Task 4 (Resources)**: Resource operations combined with JQ validation

## Test Coverage Requirements  

### Integration Tests
- Variable interpolation in JQ expressions with BDD framework
- Resource assertion helpers with fake Kubernetes clients
- Error handling and context preservation across BDD components
- Performance with realistic BDD test scenarios

### BDD Workflow Tests
- Complete scenarios: create resource → assert content → modify → assert change
- Integration with all existing BDD components
- Real-world Kubernetes resource patterns

### Compatibility Tests
- Ensure no regression in existing JQ matcher functionality
- Validate integration doesn't break existing e2e tests using JQ
- Performance comparison with direct JQ usage

## Dependencies

### Existing Packages (Reuse)
- `pkg/utils/test/matchers/jq` - **Core JQ functionality (DO NOT DUPLICATE)**
- `github.com/itchyny/gojq` - **Already integrated via existing JQ package**
- `github.com/onsi/gomega` - **Already in project**

### BDD Framework Integration  
- **Task 1**: TestContext and Config
- **Task 2**: ResourceResolver  
- **Task 3**: VariableManager
- **Task 4**: Resource CRUD operations

## Validation Criteria

1. **Leverages existing JQ infrastructure** without duplication
2. **Variable interpolation works** in JQ expressions
3. **Resource assertion helpers** simplify common BDD patterns
4. **Integration is seamless** across all BDD framework components  
5. **Performance is acceptable** and doesn't degrade existing JQ usage
6. **Error messages are enhanced** with BDD context while preserving JQ details

## Success Metrics
- [ ] BDD-specific JQ wrappers integrate seamlessly with existing matchers
- [ ] Variable interpolation works correctly in JQ expressions
- [ ] Resource assertion helpers combine CRUD operations with JQ validation
- [ ] TestContext provides fluent API for common assertion patterns
- [ ] Error handling provides both BDD context and JQ debugging details
- [ ] No performance regression compared to direct JQ usage
- [ ] Integration tests demonstrate complete BDD workflow scenarios

## Usage Examples

### Direct Integration with Existing JQ Matchers
```go
// In BDD tests, leveraging existing functionality
import "github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"

func TestPodValidation(t *testing.T) {
    g := NewWithT(t)
    ctx := setupTestContext(t)
    
    // Create resource using Task 4
    podJSON := ctx.Variables().Interpolate(`{
        "apiVersion": "v1", 
        "kind": "Pod",
        "metadata": {"name": "{{podName}}", "namespace": "{{namespace}}"},
        "spec": {"containers": [{"name": "app", "image": "{{image}}"}]}
    }`)
    
    // Use existing resource operations + existing JQ matchers
    pod := createPod(ctx, podJSON)
    g.Expect(pod).Should(jq.Match(`.spec.containers | length == 1`))
    g.Expect(pod).Should(jq.Match(`.metadata.name == "{{podName}}"`)) // With variable interpolation
}
```

### BDD-Enhanced Assertion Helpers
```go  
// Using new BDD integration layer
func TestResourceLifecycle(t *testing.T) {
    g := NewWithT(t)
    ctx := setupTestContext(t)
    
    // BDD-style fluent API
    err := ctx.AssertResource("pod", "{{podName}}", "{{namespace}}").
        Exists().
        HasValue(".status.phase", "Running").
        Matches(`.spec.containers | length == {{expectedContainers}}`)
        
    g.Expect(err).ShouldNot(HaveOccurred())
}
```

## Next Steps After Completion
Task 5 completion enables:
1. **Task 6**: Gherkin step definitions using integrated JQ assertions
2. **Complete BDD Framework**: All core components working together
3. **Advanced Test Scenarios**: Complex resource validation patterns with JQ

This task is essential for providing powerful assertion capabilities while leveraging the robust existing JQ infrastructure in the project.