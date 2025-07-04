# Task 4: Implement Resource Operations (CRUD)

## Status: 🔄 IN PROGRESS

## Overview
Implement comprehensive Kubernetes resource CRUD operations for the BDD testing framework. This task builds on Tasks 1-3 to provide robust resource management functions that work seamlessly with the TestContext, variable interpolation, and resource discovery systems.

## Files to Create/Modify

### `/pkg/utils/test/bdd/resources.go` (New)
- **Core CRUD functions** with proper error handling and validation
- **Helper functions** that compose core operations for common use cases
- **Scope validation** using Kubernetes RESTMapping
- **Unstructured object support** for generic resource operations
- **Integration** with ResourceResolver for dynamic type discovery

### `/pkg/utils/test/bdd/resources_test.go` (New)
- Comprehensive test coverage for all CRUD operations
- Edge cases for resource validation and error handling
- Integration tests with fake Kubernetes client
- Performance and concurrency testing

## Requirements from Design Document

### Core CRUD Operations
```go
// Core resource functions with context support
func CreateResource(ctx context.Context, client client.Client, obj *unstructured.Unstructured) error
func GetResource(ctx context.Context, client client.Client, gvr schema.GroupVersionResource, name string, namespace string) (*unstructured.Unstructured, error)
func UpdateResource(ctx context.Context, client client.Client, obj *unstructured.Unstructured) error
func DeleteResource(ctx context.Context, client client.Client, gvr schema.GroupVersionResource, name string, namespace string) error
```

### Helper Functions (Compose Core Functions)
```go
// Helper functions for common scenarios
func GetResourceWithDiscovery(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string, name string, namespace string) (*unstructured.Unstructured, error)
func GetSingletonResource(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string) (*unstructured.Unstructured, error)
func PatchResourceWithJQ(ctx context.Context, client client.Client, resolver *ResourceResolver, resourceType string, name string, namespace string, jqExpression string, value interface{}) error
```

### Scope Validation
```go
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

## Detailed Implementation Plan

### 1. Core CRUD Operations

#### CreateResource
- Validate unstructured object has required metadata
- Set default API version and kind if missing
- Use controller-runtime client.Create for consistent behavior
- Handle resource already exists errors appropriately
- Preserve owner references and labels

#### GetResource  
- Convert GVR to unstructured.Unstructured for client operations
- Handle namespace vs cluster-scoped resources correctly
- Return structured errors for not found vs other failures
- Support both namespaced and cluster-scoped resource retrieval

#### UpdateResource
- Preserve resourceVersion for optimistic concurrency
- Validate object has required metadata for updates
- Use controller-runtime client.Update for consistency
- Handle update conflicts with clear error messages

#### DeleteResource
- Support graceful deletion with proper propagation policy
- Handle resource not found gracefully (idempotent)
- Support both foreground and background deletion
- Return appropriate errors for permission issues

### 2. Helper Functions

#### GetResourceWithDiscovery
- Combine resource discovery and retrieval in one operation
- Resolve resource type using ResourceResolver
- Validate scope requirements automatically
- Cache resolved mappings for performance

#### GetSingletonResource
- Handle singleton resources like DSC, DSCI, FeatureTracker
- List resources and return single instance
- Handle multiple instances error case
- Support cluster-scoped singletons

#### PatchResourceWithJQ
- Combine get, JQ evaluation, and patch operations
- Support strategic merge patches
- Validate JQ expression before applying
- Handle concurrent modification gracefully

### 3. Scope Validation and Error Handling

#### ValidateResourceScope
- Use RESTMapping.Scope for authoritative scoping information
- Provide clear error messages with resource type context
- Handle edge cases for CRDs and unknown resource types
- Support validation before resource operations

#### Error Handling Strategy
- Wrap Kubernetes API errors with additional context
- Distinguish between client errors vs server errors
- Provide actionable error messages for debugging
- Support error categorization for test assertions

### 4. Integration Points

#### TestContext Integration
- Functions accept context.Context from TestContext
- Use TestContext.Client() for all operations
- Leverage TestContext.Resolver() for discovery
- Support variable interpolation in resource definitions

#### Resource Discovery Integration
- Use ResourceResolver from Task 2 for all type resolution
- Cache discovered mappings for performance
- Handle dynamic resource type discovery
- Support both built-in and custom resources

## Test Coverage Requirements

### Core CRUD Tests
- Create namespaced and cluster-scoped resources
- Get resources by various identifier patterns
- Update resources with version handling
- Delete resources with proper cleanup
- Error cases for each operation (permissions, not found, conflicts)

### Helper Function Tests
- Discovery-based resource operations
- Singleton resource handling
- JQ-based patching operations
- Complex resource lifecycle scenarios

### Edge Cases
- Resource without namespace on namespaced type
- Namespace provided for cluster-scoped type
- Invalid resource types and malformed objects
- Concurrent operations and version conflicts
- Large resource objects and performance

### Integration Tests
- End-to-end resource operations through TestContext
- Variable interpolation in resource definitions
- Resource discovery cache validation
- Error propagation through helper functions

## Dependencies

### Required Packages
- `context` (Go standard library)
- `fmt` (Go standard library)
- `k8s.io/apimachinery/pkg/apis/meta/v1/unstructured` (Kubernetes)
- `k8s.io/apimachinery/pkg/runtime/schema` (Kubernetes)
- `k8s.io/apimachinery/pkg/api/meta` (Kubernetes)
- `k8s.io/apimachinery/pkg/api/errors` (Kubernetes)
- `sigs.k8s.io/controller-runtime/pkg/client` (already in project)

### Integration Points
- **Task 1**: Uses TestContext for client access
- **Task 2**: Uses ResourceResolver for resource discovery
- **Task 3**: Supports variable interpolation in resource definitions
- **Task 5**: Will enable JQ-based resource operations

## Validation Criteria

1. **All CRUD operations work correctly** for both namespaced and cluster-scoped resources
2. **Helper functions provide value** by composing core operations effectively
3. **Error handling is comprehensive** with clear, actionable error messages
4. **Performance is acceptable** for typical BDD test scenarios
5. **Integration works seamlessly** with existing TestContext and ResourceResolver
6. **Test coverage >90%** with comprehensive edge case handling

## Success Metrics
- [ ] Core CRUD operations handle all resource types correctly
- [ ] Helper functions simplify common operations
- [ ] Scope validation prevents invalid operations
- [ ] Error handling provides clear debugging information
- [ ] Performance meets expectations for test scenarios
- [ ] Test coverage is comprehensive and reliable

## Next Steps After Completion
Task 4 completion enables:
1. **Task 5**: JQ integration for advanced resource manipulation
2. **Task 6+**: Gherkin step definitions using CRUD operations
3. **Resource Lifecycle Testing**: Complete BDD scenarios for resource management

This task is critical for providing the foundation that all Gherkin step definitions will use for Kubernetes resource operations.