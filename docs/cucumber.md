# BDD Testing Framework Design for OpenDataHub Operator

## Overview

This document outlines the design for a Behavior-Driven Development (BDD) testing framework for the OpenDataHub operator using Gherkin language and godog. The framework will enable writing cucumber-style tests for Kubernetes resources with a focus on simplicity and reusability.

## Package Structure

```
pkg/utils/test/bdd/
├── config.go          # Viper configuration management
├── context.go         # Test context and variable management
├── steps.go           # Gherkin step definitions
├── assertions.go      # JQ-based assertion logic
├── resources.go       # Kubernetes resource management
├── discovery.go       # Resource type discovery utilities
├── variables.go       # Variable interpolation and management
└── runner.go          # Test runner configuration
```

## Core Components

### 1. Configuration Management

```go
// config.go
import (
"github.com/spf13/viper"
"time"
)

// Use viper directly for configuration instead of wrapping it
func LoadBDDConfig() *viper.Viper {
v := viper.New()
v.SetDefault("eventually.timeout", 30*time.Second)
v.SetDefault("eventually.interval", 1*time.Second)
v.SetDefault("consistently.timeout", 10*time.Second)
v.SetDefault("consistently.interval", 500*time.Millisecond)
return v
}
```

### 2. Test Context

```go
// context.go
import (
"github.com/spf13/viper"
"sigs.k8s.io/controller-runtime/pkg/client"
"k8s.io/client-go/discovery"
"k8s.io/client-go/rest"
)

type TestContext struct {
config      *viper.Viper
restConfig  *rest.Config
client      client.Client
discovery   discovery.DiscoveryInterface
resolver    *ResourceResolver
variables   map[string]interface{}
}

func NewTestContext(restConfig *rest.Config) (*TestContext, error)
func (tc *TestContext) SetVariable(name string, value interface{})
func (tc *TestContext) GetVariable(name string) (interface{}, error)
func (tc *TestContext) InterpolateString(s string) (string, error)
```

### 3. Resource Discovery

```go
// discovery.go
import (
"k8s.io/apimachinery/pkg/runtime/schema"
"k8s.io/client-go/discovery"
"k8s.io/client-go/restmapper"
meta "k8s.io/apimachinery/pkg/api/meta"
)

type ResourceResolver struct {
discovery  discovery.DiscoveryInterface
restMapper meta.RESTMapper
cache      map[string]meta.RESTMapping // Use Kubernetes RESTMapping directly
}

func NewResourceResolver(discovery discovery.DiscoveryInterface) (*ResourceResolver, error)
func (r *ResourceResolver) Resolve(resourceRef string) (*meta.RESTMapping, schema.GroupVersionKind, error)
```

Supports resolution by:
- Short name (e.g., "pod", "svc", "node", "ns")
- Full name (e.g., "pod", "service", "clusterrole")
- Plural (e.g., "pods", "services", "nodes")
- Full resource name (e.g., "pods.v1", "clusterroles.rbac.authorization.k8s.io")

### 4. Variable Management

```go
// variables.go
import (
"text/template"
)

// Use standard Go maps and templates instead of custom types
func SetStaticVariable(variables map[string]interface{}, name string, value interface{})
func SetDynamicVariable(variables map[string]interface{}, name string, jqExpression string, target *unstructured.Unstructured) error
func InterpolateTemplate(tmpl string, variables map[string]interface{}) (string, error)

// Pre-defined special variables
var SpecialVariables = map[string]interface{}{
"any":       gomega.Any(),      // Use gomega's Any matcher
"ignore":    gomega.Ignore(),   // Use gomega's Ignore matcher
"timestamp": time.Now,          // Function that returns current time
}
```

### 5. JQ-based Assertions

```go
// assertions.go
import (
"github.com/onsi/gomega"
"github.com/onsi/gomega/types"
)

// Use gomega matchers directly instead of custom Assertion type
func EvaluateJQExpression(obj *unstructured.Unstructured, jqExpression string) (interface{}, error)
func MatchJQExpression(jqExpression string, expected interface{}) types.GomegaMatcher

// Helper functions for common assertions
func AssertResourceExists(ctx context.Context, client client.Client, gvr schema.GroupVersionResource, name string, namespace string) error
func AssertResourceNotExists(ctx context.Context, client client.Client, gvr schema.GroupVersionResource, name string, namespace string) error
```

### 6. Resource Operations

```go
// resources.go
import (
meta "k8s.io/apimachinery/pkg/api/meta"
)

// Core resource functions
func CreateResource(ctx context.Context, client client.Client, obj *unstructured.Unstructured) error
func GetResource(ctx context.Context, client client.Client, gvr schema.GroupVersionResource, name string, namespace string) (*unstructured.Unstructured, error)
func UpdateResource(ctx context.Context, client client.Client, obj *unstructured.Unstructured) error
func DeleteResource(ctx context.Context, client client.Client, gvr schema.GroupVersionResource, name string, namespace string) error

// Helper functions that compose the core functions
func GetResourceWithDiscovery(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string, name string, namespace string) (*unstructured.Unstructured, error)
func GetSingletonResource(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string) (*unstructured.Unstructured, error)
func PatchResourceWithJQ(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string, name string, namespace string, jqExpression string, value interface{}) error

// Scope validation using Kubernetes RESTMapping
func ValidateResourceScope(mapping *meta.RESTMapping, hasNamespace bool) error {
isNamespaced := mapping.Scope.Name() == meta.RESTScopeNamespace.Name()
if isNamespaced && !hasNamespace {
return fmt.Errorf("resource type '%s' is namespaced but no namespace was provided", mapping.GroupVersionKind.Kind)
}
if !isNamespaced && hasNamespace {
return fmt.Errorf("resource type '%s' is cluster-scoped but namespace was provided", mapping.GroupVersionKind.Kind)
}
return nil
}
```

## Gherkin Step Definitions

### Configuration Steps

```gherkin
Given the eventually timeout is "30s"
Given the consistently timeout is "10s"
Given the eventually polling interval is "1s"
Given the consistently polling interval is "500ms"
```

### Variable Management Steps

```gherkin
Given I set variable "varName" to "value"
Given I set variable "podSpec" to:
"""
  apiVersion: v1
  kind: Pod
  metadata:
    name: {{podName}}
  spec:
    containers:
    - name: main
      image: {{image}}
  """
Given I set dynamic variable "nodeCount" from expression ".status.replicas" on "deployment" named "app" in namespace "default"
And I store the result of expression ".spec" applied to the "pod" named "foo" in namespace "bar" to variable "baz"
```

### Resource Creation Steps

#### Namespaced Resources
```gherkin
When I create a "pod" named "test-pod" in namespace "default" with:
"""
  spec:
    containers:
    - name: main
      image: nginx
  """

When I create a "configmap" named "config" in namespace "prod" with:
"""
  data:
    key1: value1
    key2: value2
  """
```

#### Cluster-Scoped Resources
```gherkin
When I create a "clusterrole" named "reader" with:
"""
  rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
  """

When I create a "namespace" named "testing" with:
"""
  metadata:
    labels:
      environment: test
  """
```

#### Generic Object Creation (auto-detects scope)
```gherkin
When I create an object:
"""
  apiVersion: v1
  kind: Service
  metadata:
    name: {{serviceName}}
    namespace: {{namespace}}
  spec:
    selector:
      app: {{appName}}
    ports:
    - port: 80
  """

When I create objects from "{{podTemplate}}" with values:
| podName  | image          | replicas |
| web-1    | nginx:latest   | 3        |
| web-2    | httpd:latest   | 2        |
```

### Resource Assertion Steps

#### Namespaced Resources
```gherkin
Then the "pod" named "test-pod" in namespace "default" should exist
Then the "deployment" named "app" in namespace "prod" should eventually match expression ".status.readyReplicas == .spec.replicas"
Then the "pod" named "test-pod" in namespace "default" should consistently match expression ".status.phase == 'Running'"

Then the "deployment" named "app" in namespace "default" should have conditions:
| type            | status | reason       |
| Available       | True   | {{any}}      |
| Progressing     | True   | NewReplicaSet|

Then the "configmap" named "config" in namespace "default" should match:
| expression                | value     |
| .data.key1               | value1    |
| .data.key2               | {{ignore}} |
| .metadata.labels.version | v1.0.0    |
```

#### Cluster-Scoped Resources
```gherkin
Then the "clusterrole" named "admin" should exist
Then the "dsc" should match expression ".status.phase == 'Ready'"
Then the "node" named "worker-1" should eventually match expression ".status.conditions[?(@.type=='Ready')].status == 'True'"
Then the "crd" named "datasremember tocieneclusters.datasciencecluster.opendatahub.io" should consistently exist

Then the "clusterrolebinding" named "admin-binding" should have subjects:
| kind            | name          | namespace    |
| ServiceAccount  | admin-sa      | default      |
| User           | john@example  | {{ignore}}   |

Then the "namespace" named "production" should match:
| expression                      | value      |
| .metadata.labels.environment   | production |
| .status.phase                  | Active     |
```

### Resource Modification Steps

#### Namespaced Resources
```gherkin
When I update the "deployment" named "app" in namespace "default" setting ".spec.replicas" to 5
When I patch the "service" named "web" in namespace "prod" with expression ".spec.ports[0].port" to 8080
```

#### Cluster-Scoped Resources
```gherkin
When I update the "clusterrole" named "admin" setting ".rules[0].verbs" to ["get", "list", "watch"]
When I patch the "node" named "worker-1" with expression ".metadata.labels.zone" to "us-east-1a"
```

### Resource Deletion Steps

#### Namespaced Resources
```gherkin
When I delete the "pod" named "test-pod" in namespace "default"
Then the "pod" named "test-pod" in namespace "default" should not exist
```

#### Cluster-Scoped Resources
```gherkin
When I delete the "clusterrole" named "reader"
Then the "clusterrole" named "reader" should not exist
When I delete the "namespace" named "testing"
Then the "namespace" named "testing" should eventually not exist
```

## Implementation Tasks for AI Agent

### Task 1: Setup Package Structure and Core Types
**Files to create:**
- `pkg/utils/test/bdd/config.go`
- `pkg/utils/test/bdd/context.go`
- `pkg/utils/test/bdd/doc.go`

**Steps:**
1. Create the package directory structure `pkg/utils/test/bdd`
2. Implement `config.go` with `LoadBDDConfig()` function using viper
3. Implement `context.go` with `TestContext` struct and `NewTestContext()` function
4. Add package documentation in `doc.go`
5. Write unit tests for configuration loading

**Validation:**
- Configuration loads default values correctly
- TestContext can be created with a rest.Config
- All imports resolve correctly

### Task 2: Implement Resource Discovery
**Files to create:**
- `pkg/utils/test/bdd/discovery.go`
- `pkg/utils/test/bdd/discovery_test.go`

**Steps:**
1. Implement `ResourceResolver` struct with discovery client and REST mapper
2. Create `NewResourceResolver()` that initializes the REST mapper from discovery
3. Implement `Resolve()` method that handles:
    - Short names (pod, svc, cm)
    - Plural names (pods, services)
    - Full resource names (pods.v1)
    - Returns `*meta.RESTMapping` and `schema.GroupVersionKind`
4. Add caching to avoid repeated discovery calls
5. Write comprehensive tests using fake discovery client

**Validation:**
- Can resolve common resources (pod, service, deployment, namespace, node)
- Handles both namespaced and cluster-scoped resources
- Cache works correctly

### Task 3: Implement Variable Management
**Files to create:**
- `pkg/utils/test/bdd/variables.go`
- `pkg/utils/test/bdd/variables_test.go`

**Steps:**
1. Implement `InterpolateTemplate()` using text/template
2. Implement `SetStaticVariable()` and `SetDynamicVariable()`
3. Add special variables map with gomega matchers
4. Implement JQ evaluation for dynamic variables
5. Add template function support (e.g., `{{timestamp}}`)
6. Write tests for variable interpolation edge cases

**Validation:**
- Can interpolate simple variables: `{{name}}`
- Handles nested templates correctly
- Special variables work in assertions
- Dynamic variables evaluate JQ expressions

### Task 4: Implement Resource Operations
**Files to create:**
- `pkg/utils/test/bdd/resources.go`
- `pkg/utils/test/bdd/resources_test.go`

**Steps:**
1. Implement core CRUD functions:
    - `CreateResource()`
    - `GetResource()`
    - `UpdateResource()`
    - `DeleteResource()`
2. Implement helper functions:
    - `GetResourceWithDiscovery()`
    - `GetSingletonResource()`
    - `PatchResourceWithJQ()`
3. Implement `ValidateResourceScope()` function
4. Add proper error handling with context
5. Write tests using fake client

**Validation:**
- CRUD operations work for both namespaced and cluster resources
- Scope validation catches mismatches
- Singleton resources return error if multiple found

### Task 5: Implement JQ Assertions
**Files to create:**
- `pkg/utils/test/bdd/assertions.go`
- `pkg/utils/test/bdd/assertions_test.go`

**Steps:**
1. Integrate a JQ library (e.g., gojq)
2. Implement `EvaluateJQExpression()` function
3. Create `MatchJQExpression()` gomega matcher
4. Implement helper functions:
    - `AssertResourceExists()`
    - `AssertResourceNotExists()`
5. Handle special variables in JQ expressions
6. Write comprehensive tests for various JQ patterns

**Validation:**
- Can evaluate simple paths: `.spec.replicas`
- Handles complex queries: `.status.conditions[?(@.type=="Ready")].status`
- Works with arrays and filters
- Special variables ({{any}}, {{ignore}}) work correctly

### Task 6: Implement Basic Gherkin Steps
**Files to create:**
- `pkg/utils/test/bdd/steps.go`
- `pkg/utils/test/bdd/steps_test.go`

**Steps:**
1. Create `InitializeSteps()` function
2. Implement configuration steps:
    - Eventually/consistently timeout steps
    - Polling interval steps
3. Implement variable management steps:
    - Set static variable
    - Set dynamic variable
    - Store JQ result to variable
4. Write table-driven tests for step regex matching

**Validation:**
- Step patterns match correctly
- Configuration overrides work
- Variables are set and retrieved properly

### Task 7: Implement Resource Creation Steps
**Files to update:**
- `pkg/utils/test/bdd/steps.go`

**Steps:**
1. Implement namespaced resource creation step
2. Implement cluster-scoped resource creation step
3. Implement generic object creation step
4. Add support for multi-line YAML/JSON in doc strings
5. Implement table-based object creation step
6. Add proper error messages for YAML parsing failures

**Validation:**
- Can create pods, services, deployments with namespace
- Can create namespaces, clusterroles without namespace
- Variable interpolation works in YAML
- Table creation generates multiple objects

### Task 8: Implement Resource Assertion Steps
**Files to update:**
- `pkg/utils/test/bdd/steps.go`

**Steps:**
1. Implement existence check steps (should exist/not exist)
2. Implement JQ expression matching steps
3. Add eventually/consistently variants
4. Implement table-based assertion steps
5. Add condition checking steps
6. Handle both namespaced and cluster-scoped variants

**Validation:**
- Eventually assertions retry correctly
- Consistently assertions check over time period
- Table assertions validate multiple fields
- Timeout configuration is respected

### Task 9: Implement Resource Modification Steps
**Files to update:**
- `pkg/utils/test/bdd/steps.go`

**Steps:**
1. Implement update steps with JQ expressions
2. Implement patch steps
3. Add both namespaced and cluster-scoped variants
4. Handle complex JQ paths for nested updates
5. Add proper error handling for invalid paths

**Validation:**
- Can update simple fields like `.spec.replicas`
- Can patch array elements
- Preserves other fields during updates

### Task 10: Implement Resource Deletion Steps
**Files to update:**
- `pkg/utils/test/bdd/steps.go`

**Steps:**
1. Implement delete steps for namespaced resources
2. Implement delete steps for cluster-scoped resources
3. Add proper error handling for not found errors
4. Ensure deletion confirmation steps work

**Validation:**
- Resources are actually deleted
- Not found errors are handled gracefully
- Can verify deletion with "should not exist" steps

### Task 11: Create Test Runner and Integration
**Files to create:**
- `pkg/utils/test/bdd/runner.go`
- `pkg/utils/test/bdd/features/example.feature`

**Steps:**
1. Implement `RunBDDTests()` function
2. Set up godog test suite initialization
3. Create example feature files
4. Integrate with existing test framework
5. Add support for running specific scenarios
6. Implement test filtering options

**Validation:**
- Can run feature files
- Test results are reported correctly
- Can run individual scenarios
- Integrates with `go test`

### Task 12: Integration with OpenDataHub Types
**Files to create:**
- `pkg/utils/test/bdd/odh_helpers.go`
- `pkg/utils/test/bdd/features/datasciencecluster.feature`

**Steps:**
1. Check existing test utilities in `pkg/utils/test`
2. Create helper functions for DSC and DSCI resources
3. Write feature files for common ODH scenarios
4. Test singleton resource handling with DSC
5. Add examples for component management

**Validation:**
- Can create and validate DSC resources
- Singleton checks work for DSC
- Component status assertions work

### Task 13: Documentation and Examples
**Files to create:**
- `pkg/utils/test/bdd/README.md`
- `pkg/utils/test/bdd/examples/`

**Steps:**
1. Write comprehensive README with all available steps
2. Create example feature files for common patterns
3. Document variable usage and special variables
4. Add troubleshooting guide
5. Create quick start guide

**Validation:**
- Documentation is clear and complete
- Examples run successfully
- All steps are documented

### Task 14: Performance Optimization
**Files to update:**
- Various files for optimization

**Steps:**
1. Profile discovery operations
2. Optimize caching strategy
3. Add connection pooling if needed
4. Optimize JQ expression evaluation
5. Add benchmarks for critical paths

**Validation:**
- Discovery cache reduces API calls
- Large feature files run efficiently
- No memory leaks in long-running tests

### Task 15: Error Handling and Debugging
**Files to update:**
- All files for better error handling

**Steps:**
1. Add detailed error context to all operations
2. Implement debug logging with levels
3. Add step execution tracing
4. Improve error messages for common failures
5. Add timeout information to timeout errors

**Validation:**
- Errors clearly indicate what failed
- Debug mode provides useful information
- Timeout errors show configured values

## Example Step Implementation

```go
// steps.go - Example of how functions are composed in step definitions

func initializeSteps(ctx *godog.ScenarioContext, testCtx *TestContext) {
    // Resource creation step with namespace
    ctx.Step(`^I create a "([^"]*)" named "([^"]*)" in namespace "([^"]*)" with:# BDD Testing Framework Design for OpenDataHub Operator

## Overview

This document outlines the design for a Behavior-Driven Development (BDD) testing framework for the OpenDataHub operator using Gherkin language and godog. The framework will enable writing cucumber-style tests for Kubernetes resources with a focus on simplicity and reusability.

## Package Structure

```
pkg/utils/test/bdd/
├── config.go          # Viper configuration management
├── context.go         # Test context and variable management
├── steps.go           # Gherkin step definitions
├── assertions.go      # JQ-based assertion logic
├── resources.go       # Kubernetes resource management
├── discovery.go       # Resource type discovery utilities
├── variables.go       # Variable interpolation and management
└── runner.go          # Test runner configuration
```

## Core Components

### 1. Configuration Management

```go
// config.go
type Config struct {
    EventuallyTimeout      time.Duration
    EventuallyPollingTime  time.Duration
    ConsistentlyTimeout    time.Duration
    ConsistentlyPollingTime time.Duration
}

func LoadConfig() *Config
func (c *Config) Override(key string, value interface{})
```

Default configuration loaded via Viper with ability to override through Gherkin steps.

### 2. Test Context

```go
// context.go
type TestContext struct {
    config      *Config
    restConfig  *rest.Config
    client      client.Client
    discovery   discovery.DiscoveryInterface
    resolver    *ResourceResolver
    variables   map[string]interface{}
    resources   map[string]*unstructured.Unstructured
}

func NewTestContext(restConfig *rest.Config) (*TestContext, error)
func (tc *TestContext) SetVariable(name string, value interface{})
func (tc *TestContext) GetVariable(name string) (interface{}, error)
func (tc *TestContext) InterpolateString(s string) (string, error)
```

### 3. Resource Discovery

```go
// discovery.go
type ResourceInfo struct {
    GVR         schema.GroupVersionResource
    Namespaced  bool
}

type ResourceResolver struct {
    discovery discovery.DiscoveryInterface
    cache     map[string]ResourceInfo
}

func NewResourceResolver(discovery discovery.DiscoveryInterface) *ResourceResolver
func (r *ResourceResolver) Resolve(resourceRef string) (ResourceInfo, error)
func (r *ResourceResolver) IsNamespaced(resourceRef string) (bool, error)
```

Supports resolution by:
- Short name (e.g., "pod", "svc", "node", "ns")
- Full name (e.g., "pod", "service", "clusterrole")
- Plural (e.g., "pods", "services", "nodes")
- Full resource name (e.g., "pods.v1", "clusterroles.rbac.authorization.k8s.io")

### 4. Variable Management

```go
// variables.go
type VariableManager struct {
    static  map[string]interface{}
    dynamic map[string]string // JQ expressions
}

func (vm *VariableManager) Set(name string, value interface{})
func (vm *VariableManager) SetDynamic(name string, jqExpression string, target *unstructured.Unstructured)
func (vm *VariableManager) Interpolate(template string) (string, error)
func (vm *VariableManager) EvaluateDynamic(ctx *TestContext) error
```

Pre-defined special variables:
- `{{any}}` - Matches any value in assertions
- `{{ignore}}` - Ignores the field in assertions
- `{{namespace}}` - Current test namespace
- `{{timestamp}}` - Current timestamp

### 5. JQ-based Assertions

```go
// assertions.go
type Assertion struct {
    jqExpression string
    expected     interface{}
}

func NewAssertion(expression string, expected interface{}) *Assertion
func (a *Assertion) Evaluate(obj *unstructured.Unstructured) (bool, error)
func (a *Assertion) EvaluateEventually(ctx *TestContext, obj *unstructured.Unstructured) error
func (a *Assertion) EvaluateConsistently(ctx *TestContext, obj *unstructured.Unstructured) error
```

### 6. Resource Operations

```go
// resources.go

// Core resource functions
func CreateResource(ctx context.Context, client client.Client, obj *unstructured.Unstructured) error
func GetResource(ctx context.Context, client client.Client, gvr schema.GroupVersionResource, name string, namespace string) (*unstructured.Unstructured, error)
func UpdateResource(ctx context.Context, client client.Client, obj *unstructured.Unstructured) error
func DeleteResource(ctx context.Context, client client.Client, gvr schema.GroupVersionResource, name string, namespace string) error

// Helper functions that compose the core functions
func GetResourceWithDiscovery(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string, name string, namespace string) (*unstructured.Unstructured, error)
func GetSingletonResource(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string) (*unstructured.Unstructured, error)
func PatchResourceWithJQ(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string, name string, namespace string, jqExpression string, value interface{}) error

// Scope validation
func ValidateResourceScope(resourceInfo ResourceInfo, hasNamespace bool) error
func RequiresNamespace(resourceInfo ResourceInfo) bool
```

## Gherkin Step Definitions

### Configuration Steps

```gherkin
Given the eventually timeout is "30s"
Given the consistently timeout is "10s"
Given the eventually polling interval is "1s"
Given the consistently polling interval is "500ms"
```

### Variable Management Steps

```gherkin
Given I set variable "varName" to "value"
Given I set variable "podSpec" to:
  """
  apiVersion: v1
  kind: Pod
  metadata:
    name: {{podName}}
  spec:
    containers:
    - name: main
      image: {{image}}
  """
Given I set dynamic variable "nodeCount" from expression ".status.replicas" on "deployment" named "app" in namespace "default"
And I store the result of expression ".spec" applied to the "pod" named "foo" in namespace "bar" to variable "baz"
```

### Resource Creation Steps

#### Namespaced Resources
```gherkin
When I create a "pod" named "test-pod" in namespace "default" with:
  """
  spec:
    containers:
    - name: main
      image: nginx
  """

When I create a "configmap" named "config" in namespace "prod" with:
  """
  data:
    key1: value1
    key2: value2
  """
```

#### Cluster-Scoped Resources
```gherkin
When I create a "clusterrole" named "reader" with:
  """
  rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
  """

When I create a "namespace" named "testing" with:
  """
  metadata:
    labels:
      environment: test
  """
```

#### Generic Object Creation (auto-detects scope)
```gherkin
When I create an object:
  """
  apiVersion: v1
  kind: Service
  metadata:
    name: {{serviceName}}
    namespace: {{namespace}}
  spec:
    selector:
      app: {{appName}}
    ports:
    - port: 80
  """

When I create objects from "{{podTemplate}}" with values:
  | podName  | image          | replicas |
  | web-1    | nginx:latest   | 3        |
  | web-2    | httpd:latest   | 2        |
```

### Resource Assertion Steps

#### Namespaced Resources
```gherkin
Then the "pod" named "test-pod" in namespace "default" should exist
Then the "deployment" named "app" in namespace "prod" should eventually match expression ".status.readyReplicas == .spec.replicas"
Then the "pod" named "test-pod" in namespace "default" should consistently match expression ".status.phase == 'Running'"

Then the "deployment" named "app" in namespace "default" should have conditions:
  | type            | status | reason       |
  | Available       | True   | {{any}}      |
  | Progressing     | True   | NewReplicaSet|

Then the "configmap" named "config" in namespace "default" should match:
  | expression                | value     |
  | .data.key1               | value1    |
  | .data.key2               | {{ignore}} |
  | .metadata.labels.version | v1.0.0    |
```

#### Cluster-Scoped Resources
```gherkin
Then the "clusterrole" named "admin" should exist
Then the "dsc" should match expression ".status.phase == 'Ready'"
Then the "node" named "worker-1" should eventually match expression ".status.conditions[?(@.type=='Ready')].status == 'True'"
Then the "crd" named "datascieneclusters.datasciencecluster.opendatahub.io" should consistently exist

Then the "clusterrolebinding" named "admin-binding" should have subjects:
  | kind            | name          | namespace    |
  | ServiceAccount  | admin-sa      | default      |
  | User           | john@example  | {{ignore}}   |

Then the "namespace" named "production" should match:
  | expression                      | value      |
  | .metadata.labels.environment   | production |
  | .status.phase                  | Active     |
```

### Resource Modification Steps

#### Namespaced Resources
```gherkin
When I update the "deployment" named "app" in namespace "default" setting ".spec.replicas" to 5
When I patch the "service" named "web" in namespace "prod" with expression ".spec.ports[0].port" to 8080
```

#### Cluster-Scoped Resources
```gherkin
When I update the "clusterrole" named "admin" setting ".rules[0].verbs" to ["get", "list", "watch"]
When I patch the "node" named "worker-1" with expression ".metadata.labels.zone" to "us-east-1a"
```

### Resource Deletion Steps

#### Namespaced Resources
```gherkin
When I delete the "pod" named "test-pod" in namespace "default"
Then the "pod" named "test-pod" in namespace "default" should not exist
```

#### Cluster-Scoped Resources
```gherkin
When I delete the "clusterrole" named "reader"
Then the "clusterrole" named "reader" should not exist
When I delete the "namespace" named "testing"
Then the "namespace" named "testing" should eventually not exist
```

,
func(resourceType, name, namespace string, docString *godog.DocString) error {
// Resolve resource type
resourceInfo, err := testCtx.resolver.Resolve(resourceType)
if err != nil {
return fmt.Errorf("failed to resolve resource type %s: %w", resourceType, err)
}

            // Validate scope
            if err := ValidateResourceScope(resourceInfo, true); err != nil {
                return err
            }
            
            // Parse and interpolate the YAML
            interpolated, err := testCtx.InterpolateString(docString.Content)
            if err != nil {
                return fmt.Errorf("failed to interpolate variables: %w", err)
            }
            
            // Create unstructured object
            obj := &unstructured.Unstructured{}
            if err := yaml.Unmarshal([]byte(interpolated), obj); err != nil {
                return fmt.Errorf("failed to parse YAML: %w", err)
            }
            
            // Set metadata
            obj.SetName(name)
            obj.SetNamespace(namespace)
            obj.SetGroupVersionKind(schema.GroupVersionKind{
                Group:   resourceInfo.GVR.Group,
                Version: resourceInfo.GVR.Version,
                Kind:    resourceType, // This would need proper kind resolution
            })
            
            // Create the resource
            return CreateResource(context.Background(), testCtx.client, obj)
        })
    
    // Assertion step with Eventually
    ctx.Step(`^the "([^"]*)" named "([^"]*)" in namespace "([^"]*)" should eventually match expression "([^"]*)"# BDD Testing Framework Design for OpenDataHub Operator

## Overview

This document outlines the design for a Behavior-Driven Development (BDD) testing framework for the OpenDataHub operator using Gherkin language and godog. The framework will enable writing cucumber-style tests for Kubernetes resources with a focus on simplicity and reusability.

## Package Structure

```
pkg/utils/test/bdd/
├── config.go          # Viper configuration management
├── context.go         # Test context and variable management
├── steps.go           # Gherkin step definitions
├── assertions.go      # JQ-based assertion logic
├── resources.go       # Kubernetes resource management
├── discovery.go       # Resource type discovery utilities
├── variables.go       # Variable interpolation and management
└── runner.go          # Test runner configuration
```

## Core Components

### 1. Configuration Management

```go
// config.go
type Config struct {
    EventuallyTimeout      time.Duration
    EventuallyPollingTime  time.Duration
    ConsistentlyTimeout    time.Duration
    ConsistentlyPollingTime time.Duration
}

func LoadConfig() *Config
func (c *Config) Override(key string, value interface{})
```

Default configuration loaded via Viper with ability to override through Gherkin steps.

### 2. Test Context

```go
// context.go
type TestContext struct {
    config      *Config
    restConfig  *rest.Config
    client      client.Client
    discovery   discovery.DiscoveryInterface
    resolver    *ResourceResolver
    variables   map[string]interface{}
    resources   map[string]*unstructured.Unstructured
}

func NewTestContext(restConfig *rest.Config) (*TestContext, error)
func (tc *TestContext) SetVariable(name string, value interface{})
func (tc *TestContext) GetVariable(name string) (interface{}, error)
func (tc *TestContext) InterpolateString(s string) (string, error)
```

### 3. Resource Discovery

```go
// discovery.go
type ResourceInfo struct {
    GVR         schema.GroupVersionResource
    Namespaced  bool
}

type ResourceResolver struct {
    discovery discovery.DiscoveryInterface
    cache     map[string]ResourceInfo
}

func NewResourceResolver(discovery discovery.DiscoveryInterface) *ResourceResolver
func (r *ResourceResolver) Resolve(resourceRef string) (ResourceInfo, error)
func (r *ResourceResolver) IsNamespaced(resourceRef string) (bool, error)
```

Supports resolution by:
- Short name (e.g., "pod", "svc", "node", "ns")
- Full name (e.g., "pod", "service", "clusterrole")
- Plural (e.g., "pods", "services", "nodes")
- Full resource name (e.g., "pods.v1", "clusterroles.rbac.authorization.k8s.io")

### 4. Variable Management

```go
// variables.go
type VariableManager struct {
    static  map[string]interface{}
    dynamic map[string]string // JQ expressions
}

func (vm *VariableManager) Set(name string, value interface{})
func (vm *VariableManager) SetDynamic(name string, jqExpression string, target *unstructured.Unstructured)
func (vm *VariableManager) Interpolate(template string) (string, error)
func (vm *VariableManager) EvaluateDynamic(ctx *TestContext) error
```

Pre-defined special variables:
- `{{any}}` - Matches any value in assertions
- `{{ignore}}` - Ignores the field in assertions
- `{{namespace}}` - Current test namespace
- `{{timestamp}}` - Current timestamp

### 5. JQ-based Assertions

```go
// assertions.go
type Assertion struct {
    jqExpression string
    expected     interface{}
}

func NewAssertion(expression string, expected interface{}) *Assertion
func (a *Assertion) Evaluate(obj *unstructured.Unstructured) (bool, error)
func (a *Assertion) EvaluateEventually(ctx *TestContext, obj *unstructured.Unstructured) error
func (a *Assertion) EvaluateConsistently(ctx *TestContext, obj *unstructured.Unstructured) error
```

### 6. Resource Operations

```go
// resources.go

// Core resource functions
func CreateResource(ctx context.Context, client client.Client, obj *unstructured.Unstructured) error
func GetResource(ctx context.Context, client client.Client, gvr schema.GroupVersionResource, name string, namespace string) (*unstructured.Unstructured, error)
func UpdateResource(ctx context.Context, client client.Client, obj *unstructured.Unstructured) error
func DeleteResource(ctx context.Context, client client.Client, gvr schema.GroupVersionResource, name string, namespace string) error

// Helper functions that compose the core functions
func GetResourceWithDiscovery(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string, name string, namespace string) (*unstructured.Unstructured, error)
func GetSingletonResource(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string) (*unstructured.Unstructured, error)
func PatchResourceWithJQ(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string, name string, namespace string, jqExpression string, value interface{}) error

// Scope validation
func ValidateResourceScope(resourceInfo ResourceInfo, hasNamespace bool) error
func RequiresNamespace(resourceInfo ResourceInfo) bool
```

## Gherkin Step Definitions

### Configuration Steps

```gherkin
Given the eventually timeout is "30s"
Given the consistently timeout is "10s"
Given the eventually polling interval is "1s"
Given the consistently polling interval is "500ms"
```

### Variable Management Steps

```gherkin
Given I set variable "varName" to "value"
Given I set variable "podSpec" to:
  """
  apiVersion: v1
  kind: Pod
  metadata:
    name: {{podName}}
  spec:
    containers:
    - name: main
      image: {{image}}
  """
Given I set dynamic variable "nodeCount" from expression ".status.replicas" on "deployment" named "app" in namespace "default"
And I store the result of expression ".spec" applied to the "pod" named "foo" in namespace "bar" to variable "baz"
```

### Resource Creation Steps

#### Namespaced Resources
```gherkin
When I create a "pod" named "test-pod" in namespace "default" with:
  """
  spec:
    containers:
    - name: main
      image: nginx
  """

When I create a "configmap" named "config" in namespace "prod" with:
  """
  data:
    key1: value1
    key2: value2
  """
```

#### Cluster-Scoped Resources
```gherkin
When I create a "clusterrole" named "reader" with:
  """
  rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
  """

When I create a "namespace" named "testing" with:
  """
  metadata:
    labels:
      environment: test
  """
```

#### Generic Object Creation (auto-detects scope)
```gherkin
When I create an object:
  """
  apiVersion: v1
  kind: Service
  metadata:
    name: {{serviceName}}
    namespace: {{namespace}}
  spec:
    selector:
      app: {{appName}}
    ports:
    - port: 80
  """

When I create objects from "{{podTemplate}}" with values:
  | podName  | image          | replicas |
  | web-1    | nginx:latest   | 3        |
  | web-2    | httpd:latest   | 2        |
```

### Resource Assertion Steps

#### Namespaced Resources
```gherkin
Then the "pod" named "test-pod" in namespace "default" should exist
Then the "deployment" named "app" in namespace "prod" should eventually match expression ".status.readyReplicas == .spec.replicas"
Then the "pod" named "test-pod" in namespace "default" should consistently match expression ".status.phase == 'Running'"

Then the "deployment" named "app" in namespace "default" should have conditions:
  | type            | status | reason       |
  | Available       | True   | {{any}}      |
  | Progressing     | True   | NewReplicaSet|

Then the "configmap" named "config" in namespace "default" should match:
  | expression                | value     |
  | .data.key1               | value1    |
  | .data.key2               | {{ignore}} |
  | .metadata.labels.version | v1.0.0    |
```

#### Cluster-Scoped Resources
```gherkin
Then the "clusterrole" named "admin" should exist
Then the "dsc" should match expression ".status.phase == 'Ready'"
Then the "node" named "worker-1" should eventually match expression ".status.conditions[?(@.type=='Ready')].status == 'True'"
Then the "crd" named "datascieneclusters.datasciencecluster.opendatahub.io" should consistently exist

Then the "clusterrolebinding" named "admin-binding" should have subjects:
  | kind            | name          | namespace    |
  | ServiceAccount  | admin-sa      | default      |
  | User           | john@example  | {{ignore}}   |

Then the "namespace" named "production" should match:
  | expression                      | value      |
  | .metadata.labels.environment   | production |
  | .status.phase                  | Active     |
```

### Resource Modification Steps

#### Namespaced Resources
```gherkin
When I update the "deployment" named "app" in namespace "default" setting ".spec.replicas" to 5
When I patch the "service" named "web" in namespace "prod" with expression ".spec.ports[0].port" to 8080
```

#### Cluster-Scoped Resources
```gherkin
When I update the "clusterrole" named "admin" setting ".rules[0].verbs" to ["get", "list", "watch"]
When I patch the "node" named "worker-1" with expression ".metadata.labels.zone" to "us-east-1a"
```

### Resource Deletion Steps

#### Namespaced Resources
```gherkin
When I delete the "pod" named "test-pod" in namespace "default"
Then the "pod" named "test-pod" in namespace "default" should not exist
```

#### Cluster-Scoped Resources
```gherkin
When I delete the "clusterrole" named "reader"
Then the "clusterrole" named "reader" should not exist
When I delete the "namespace" named "testing"
Then the "namespace" named "testing" should eventually not exist
```

,
func(resourceType, name, namespace, jqExpression string) error {
return gomega.Eventually(func() (bool, error) {
obj, err := GetResourceWithDiscovery(
context.Background(),
testCtx.client,
testCtx.resolver,
resourceType,
name,
namespace,
)
if err != nil {
return false, err
}

                assertion := NewAssertion(jqExpression, true)
                return assertion.Evaluate(obj)
            }, testCtx.config.EventuallyTimeout, testCtx.config.EventuallyPollingTime).Should(gomega.BeTrue())
        })
}
```

## Integration with OpenDataHub Operator

### Example Test Scenario

```gherkin
Feature: DataScienceCluster Management
  Background:
    Given the eventually timeout is "5m"
    And I set variable "namespace" to "opendatahub"
    
  Scenario: Create and validate DSC
    When I create an object:
      """
      apiVersion: dscinitialization.opendatahub.io/v1
      kind: DSCInitialization
      metadata:
        name: default
      spec:
        applicationsNamespace: {{namespace}}
      """
    Then the "dsci" should eventually match expression ".status.phase == 'Ready'"
    
    When I create an object:
      """
      apiVersion: datasciencecluster.opendatahub.io/v1
      kind: DataScienceCluster
      metadata:
        name: default
      spec:
        components:
          dashboard:
            managementState: Managed
      """
    Then the "dsc" should eventually have conditions:
      | type     | status | reason     |
      | Ready    | True   | Configured |
    And the "dsc" should match expression ".spec.components.dashboard.managementState == 'Managed'"
```

## Notes for Implementation

- Leverage existing helpers from `pkg/utils/test/matchers` and `pkg/utils/test/testf`
- Use standard Kubernetes types from `k8s.io/api` and `k8s.io/apimachinery`
- Keep functions focused and composable
- Avoid creating unnecessary abstractions
- Use `controller-runtime` client for resource operations
- Implement proper error handling with context
- Make step definitions as readable as natural language

### Scope Validation
- When a namespace is provided in a step, validate that the resource type is namespaced
- When no namespace is provided, validate that the resource type is cluster-scoped
- Fail fast with clear error messages like: "Error: 'clusterrole' is a cluster-scoped resource but namespace 'default' was provided"
- Use discovery API to determine resource scope at runtime
- Cache scope information to avoid repeated API calls

## Additional Considerations

### Error Handling
- All steps should provide clear error messages
- Include resource type, name, and namespace in error contexts
- Wrap errors with additional context using `fmt.Errorf`

### Performance
- Cache discovery results to avoid repeated API calls
- Use field selectors when possible for singleton resources
- Implement resource watching for long-running assertions

### Extensibility
- Allow custom step definitions to be registered
- Provide hooks for before/after scenario execution
- Support custom JQ functions for domain-specific logic

### Integration Points
- Reuse existing test utilities from the codebase
- Integrate with existing logging infrastructure
- Support test result reporting in standard formats 