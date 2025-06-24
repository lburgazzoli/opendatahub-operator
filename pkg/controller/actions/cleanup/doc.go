/*
Package cleanup provides actions for cleaning up Kubernetes resources managed by the OpenDataHub operator.

The cleanup package implements a generic cleanup mechanism that can discover, filter, and clean up
Kubernetes resources based on configurable criteria. It provides safe cleanup operations with
built-in protections for critical resource types and proper authorization checks.

# Core Components

The main components of the cleanup package are:

- Action: The primary cleanup action that orchestrates the cleanup process
- HandlerFn: Functions that define how specific resources should be cleaned up
- TypePredicateFn: Functions that determine whether a resource type can be cleaned up
- ActionOpts: Configuration options for customizing cleanup behavior

# Basic Usage

The simplest way to create a cleanup action is:

	cleanupAction := cleanup.NewAction()

This creates a cleanup action with default settings that will:
- Clean up resources in the operator namespace
- Use label selector based on platform.opendatahub.io/part-of
- Protect CRDs and Leases from deletion
- Use foreground deletion propagation

# Configuration Options

The cleanup action can be customized using various options:

	cleanupAction := cleanup.NewAction(
		cleanup.WithLabel("app", "my-component"),
		cleanup.WithProtectedTypes(schema.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		}),
		cleanup.InNamespace("my-namespace"),
		cleanup.WithHandler(myCustomHandler),
		cleanup.WithTypePredicate(myTypePredicate),
	)

# Label-based Selection

Resources are selected for cleanup using Kubernetes label selectors. You can specify custom labels:

	// Clean up resources with specific labels
	cleanup.NewAction(
		cleanup.WithLabel("component", "dashboard"),
		cleanup.WithLabel("version", "v1.0.0"),
	)

	// Or use a map of labels
	cleanup.NewAction(
		cleanup.WithLabels(map[string]string{
			"app":     "my-app",
			"version": "v2.0.0",
		}),
	)

# Namespace for Authorization Checks

The cleanup action needs to determine which namespace to use for self subject access review calls to check if the operator has permission to delete resources. By default, it uses the operator namespace, but you can customize this:

	// Use a specific namespace for authorization checks
	cleanup.NewAction(cleanup.InNamespace("my-namespace"))

	// Use a function to determine the namespace for authorization checks dynamically
	cleanup.NewAction(cleanup.InNamespaceFn(func(ctx context.Context, rr *odhTypes.ReconciliationRequest) (string, error) {
		return rr.Instance.GetNamespace(), nil
	}))

# Protected Types

Certain resource types are protected from cleanup by default (CRDs, Leases). You can add additional protected types:

	cleanup.NewAction(
		cleanup.WithProtectedTypes(
			schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
		),
	)

# Cleanup Handlers

The cleanup package provides several built-in handlers for different cleanup strategies:

## DeleteHandler

The DeleteHandler physically deletes resources from the cluster. It supports various options for fine-grained control:

	// Basic delete handler with foreground propagation
	cleanup.NewAction(
		cleanup.WithHandler(cleanup.DefaultDeleteHandler(metav1.DeletePropagationForeground)),
	)

	// Custom delete handler with options
	cleanup.NewAction(
		cleanup.WithHandler(cleanup.DeleteHandler(
			cleanup.WithPropagationPolicy(metav1.DeletePropagationBackground),
			cleanup.WithOwnershipCheck(true),          // Only delete resources owned by the instance
			cleanup.WithManagedAnnotationCheck(true),  // Skip resources with managed=false annotation
		)),
	)

### DeleteHandler Options:

- **PropagationPolicy**: Controls how deletion cascades (Foreground, Background, Orphan)
- **OwnershipCheck**: When enabled, only deletes resources owned by the reconciling instance
- **ManagedAnnotationCheck**: When enabled, skips resources with `platform.opendatahub.io/managed: "false"`
- **ObjectPredicate**: Custom function to determine if a specific object should be deleted

## DeownHandler

The DeownHandler removes owner references without deleting the actual resources. This is useful when you want to stop managing resources but leave them in the cluster:

	// Basic deown handler
	cleanup.NewAction(
		cleanup.WithHandler(cleanup.DeownHandler()),
	)

	// Deown handler with custom options
	cleanup.NewAction(
		cleanup.WithHandler(cleanup.DeownHandler(
			cleanup.WithManagedAnnotationCheck(false), // Process all resources regardless of managed annotation
			cleanup.WithObjectPredicate(myCustomPredicate),
		)),
	)

### DeownHandler Options:

- **ManagedAnnotationCheck**: When enabled, skips resources with `platform.opendatahub.io/managed: "false"`
- **ObjectPredicate**: Custom function to determine if a specific object should be processed

## Custom Handlers

You can also provide completely custom cleanup logic:

	customHandler := func(ctx context.Context, rr *odhTypes.ReconciliationRequest, obj unstructured.Unstructured) (bool, error) {
		// Custom cleanup logic here
		if obj.GetName() == "keep-this-resource" {
			return false, nil // Don't clean up this resource
		}
		
		// Perform custom cleanup - could be deletion, modification, etc.
		err := rr.Client.Delete(ctx, &obj)
		return true, err
	}

	cleanup.NewAction(cleanup.WithHandler(customHandler))

## Handler Selection Guidelines

- **Use DeleteHandler** when you want to completely remove resources during cleanup
- **Use DeownHandler** when you want to stop managing resources but keep them in the cluster
- **Use Custom Handlers** when you need specialized cleanup logic (e.g., graceful shutdown, data backup, etc.)

# Type Predicates

Type predicates allow you to filter which resource types should be considered for cleanup:

	myPredicate := func(rr *odhTypes.ReconciliationRequest, gvk schema.GroupVersionKind) (bool, error) {
		// Only clean up ConfigMaps and Secrets
		return gvk.Kind == "ConfigMap" || gvk.Kind == "Secret", nil
	}

	cleanup.NewAction(cleanup.WithTypePredicate(myPredicate))

# Complete Example

Here's a complete example of setting up a cleanup action for a component:

	func createComponentCleanupAction() actions.Fn {
		return cleanup.NewAction(
			// Target resources with specific labels
			cleanup.WithLabels(map[string]string{
				"app.kubernetes.io/name":      "my-component",
				"app.kubernetes.io/instance":  "my-instance",
			}),
			
			// Use component's namespace for authorization checks
			cleanup.InNamespaceFn(func(ctx context.Context, rr *odhTypes.ReconciliationRequest) (string, error) {
				return rr.Instance.GetNamespace(), nil
			}),
			
			// Protect important resources
			cleanup.WithProtectedTypes(
				schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			),
			
			// Custom cleanup logic
			cleanup.WithHandler(func(ctx context.Context, rr *odhTypes.ReconciliationRequest, obj unstructured.Unstructured) (bool, error) {
				log := logf.FromContext(ctx)
				
				// Log what we're cleaning up
				log.Info("Cleaning up resource", 
					"kind", obj.GetKind(), 
					"name", obj.GetName(),
					"namespace", obj.GetNamespace(),
				)
				
				// Use default deletion with background propagation
				return true, rr.Client.Delete(ctx, &obj, client.PropagationPolicy(metav1.DeletePropagationBackground))
			}),
		)
	}

# Safety Features

The cleanup package includes several safety features:

- Authorization checks ensure the operator has permission to delete resources
- Resources already being deleted (with deletion timestamp) are skipped
- Protected types cannot be cleaned up regardless of labels
- Cleanup only runs when resources have been generated (rr.Generated == true)
- Proper error handling and logging throughout the process

# Integration with Reconcilers

Cleanup actions are typically integrated with reconcilers in two ways:

1. **Using GC Action (Recommended)** - The standard approach using the gc package:

	func (s *componentHandler) NewComponentReconciler(ctx context.Context, mgr ctrl.Manager) error {
		_, err := reconciler.ReconcilerFor(mgr, &componentApi.MyComponent{}).
			WithAction(initialize).
			WithAction(deploy.NewAction()).
			// GC action handles cleanup automatically based on labels
			WithAction(gc.NewAction()).
			Build(ctx)
		
		return err
	}

2. **Custom Cleanup Action** - For specialized cleanup logic, you can inline the configuration:

	func (s *componentHandler) NewComponentReconciler(ctx context.Context, mgr ctrl.Manager) error {
		_, err := reconciler.ReconcilerFor(mgr, &componentApi.MyComponent{}).
			WithAction(initialize).
			WithAction(deploy.NewAction()).
			// Custom cleanup action with specific configuration
			WithAction(cleanup.NewAction(
				cleanup.WithLabels(map[string]string{
					"app.kubernetes.io/name": "my-component",
					"app.kubernetes.io/part-of": "opendatahub",
				}),
				cleanup.InNamespaceFn(func(ctx context.Context, rr *odhTypes.ReconciliationRequest) (string, error) {
					return rr.Instance.GetNamespace(), nil
				}),
				cleanup.WithHandler(cleanup.DefaultDeleteHandler(metav1.DeletePropagationBackground)),
			)).
			Build(ctx)
		
		return err
	}
*/
package cleanup