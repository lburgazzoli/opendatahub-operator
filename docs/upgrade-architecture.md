# Upgrade Architecture

Related: [RHOAIENG-48390](https://issues.redhat.com/browse/RHOAIENG-48390)

Upgrade logic is distributed across component-specific controllers rather than running monolithically at startup. This provides:
- **Isolation**: One component's upgrade failure doesn't block others
- **Visibility**: Progress visible through component conditions
- **Maintainability**: Upgrade code lives with the component it affects

## Architecture

```
                    ┌─────────────────────────────────────┐
                    │         DSC/DSCI Controllers        │
                    │  (cross-cutting upgrades only)      │
                    └─────────────────────────────────────┘
                                      │
          ┌───────────────────────────┼───────────────────────────┐
          ▼                           ▼                           ▼
   ┌─────────────┐            ┌─────────────┐            ┌─────────────┐
   │   Kueue     │            │  Dashboard  │            │   KServe    │
   │ Controller  │            │ Controller  │            │ Controller  │
   │ + upgrade   │            │ + upgrade   │            │ + upgrade   │
   └─────────────┘            └─────────────┘            └─────────────┘
```

## Migration Map

| Function | Controller | Rationale |
|----------|------------|-----------|
| `cleanupDeprecatedVAPB` | Kueue | Kueue-specific resource |
| `cleanupLegacyDeployment` | ModelController | MC-specific deployment |
| `migrateHardwareProfiles` | DSC | Cross-cutting, spans multiple components |
| `cleanupDeprecatedRoleBindings` | DSC | Cross-cutting infrastructure |
| `migrateIngressMode` | Gateway | Gateway-specific migration |

## Using `upgrade.Action`

The `upgrade.Action` wrapper provides:
- Thread-safe run-once semantics
- Automatic `StopError` wrapping on failure (blocking mode)
- Standard error return (non-blocking mode)
- All functions run and errors are combined
- Retry on next reconcile if failed

### Upgrade Function Signature

```go
// Fn is the signature for upgrade functions.
// cli is the client for both reads and writes.
type Fn func(ctx context.Context, cli client.Client) error
```

### Functional Options

The action supports functional options for configuration:

```go
// WithFn adds an upgrade function to execute.
upgrade.WithFn(myUpgradeFunction)

// NonBlocking makes the action non-blocking.
// On failure: logs error, returns standard error (not StopError).
upgrade.NonBlocking()
```

### Controller Integration

**Component Controllers (blocking, default behavior):**

```go
func myUpgrade(ctx context.Context, cli client.Client) error {
    obj := &SomeResource{}
    err := cli.Get(ctx, key, obj)
    switch {
    case k8serr.IsNotFound(err):
        return nil
    case err != nil:
        return fmt.Errorf("failed to get resource: %w", err)
    }

    err = cli.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationForeground))
    switch {
    case k8serr.IsNotFound(err):
        return nil
    case err != nil:
        return fmt.Errorf("failed to delete resource: %w", err)
    }
    return nil
}

func (s *handler) NewComponentReconciler(ctx context.Context, mgr ctrl.Manager) error {
    // Create action as local variable - no global variables needed
    myUpgradeAction := upgrade.NewAction(mgr,
        upgrade.WithFn(myUpgrade),
    )

    return reconciler.ReconcilerFor(mgr, &componentApi.MyComponent{}).
        WithAction(myUpgradeAction.Run). // First action in chain
        WithAction(initialize).
        // ...
        Build(ctx)
}
```

**DSC/DSCI Controllers (non-blocking):**

For parent controllers like DSC, use `NonBlocking()` to prevent upgrade failures from stopping the reconciliation chain:

```go
func NewDSCReconciler(ctx context.Context, mgr ctrl.Manager) error {
    // Non-blocking: failures return standard error but don't stop the chain
    upgradeAction := upgrade.NewAction(mgr,
        upgrade.WithFn(migrateHardwareProfiles),
        upgrade.WithFn(cleanupDeprecatedRoleBindings),
        upgrade.NonBlocking(),
    )

    return reconciler.ReconcilerFor(mgr, &dscv2.DataScienceCluster{}).
        WithAction(upgradeAction.Run).
        // ... other actions ...
        WithConditions(status.ConditionTypeComponentsReady).
        Build(ctx)
}
```

## Blocking vs Non-Blocking Mode

| Mode | Behavior on Error | Use Case |
|------|------------------|----------|
| **Blocking (default)** | Returns `StopError`, stops reconciliation chain | Component controllers (Kueue, ModelController, Gateway) - failures only affect that component |
| **Non-Blocking** | Returns standard error, marks provisioning failed but chain continues | DSC controller - must not block child component provisioning |

### Error Combining

All upgrade functions run regardless of individual failures. Errors are combined using `errors.Join()`:

```go
// If fn1 succeeds, fn2 fails, fn3 fails:
// - All three functions execute
// - Combined error contains both fn2 and fn3 errors
// - Action retries all functions on next reconcile
```

## Key Points

- Use `upgrade.NewAction(mgr, upgrade.WithFn(fn))` - creates non-caching client internally
- Avoid global variables - create action as local variable in controller setup
- Upgrade actions are always the **first** action in the chain
- Handle `NotFound` and `NoMatch` errors gracefully (skip, don't fail)
- Use switch case for all k8s API error handling
- Always wrap errors with context

## Error Handling

**Component Controllers** (Kueue, ModelController, Gateway):

Return errors directly - `upgrade.Action` wraps to `StopError` automatically:

```go
func myUpgrade(ctx context.Context, cli client.Client) error {
    err := cli.Get(ctx, key, obj)
    switch {
    case k8serr.IsNotFound(err):
        return nil
    case err != nil:
        return fmt.Errorf("failed to get resource: %w", err)
    }

    err = cli.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationForeground))
    switch {
    case k8serr.IsNotFound(err):
        return nil
    case err != nil:
        return fmt.Errorf("failed to delete resource: %w", err)
    }
    return nil
}
```

**DSC/DSCI Controllers**:

Same pattern as component controllers, but use `NonBlocking()` option. Errors are logged and returned as standard errors:

```go
func NewDSCReconciler(ctx context.Context, mgr ctrl.Manager) error {
    upgradeAction := upgrade.NewAction(mgr,
        upgrade.WithFn(migrateHardwareProfiles),
        upgrade.NonBlocking(),
    )

    return reconciler.ReconcilerFor(mgr, &dscv2.DataScienceCluster{}).
        WithAction(upgradeAction.Run).
        // ...
        Build(ctx)
}
```

## Idempotent Operations

Upgrade actions must be idempotent and handle missing resources gracefully:

```go
func myUpgrade(ctx context.Context, cli client.Client) error {
    // List - use switch case, skip NoMatch (CRD missing) and NotFound
    list := &unstructured.UnstructuredList{}
    list.SetGroupVersionKind(gvk.SomeResource)
    err := cli.List(ctx, list)
    switch {
    case k8serr.IsNotFound(err), meta.IsNoMatchError(err):
        return nil
    case err != nil:
        return fmt.Errorf("failed to list resources: %w", err)
    case len(list.Items) == 0:
        return nil
    }

    // Get - use switch case, skip NotFound and NoMatch
    err = cli.Get(ctx, key, obj)
    switch {
    case k8serr.IsNotFound(err), meta.IsNoMatchError(err):
        return nil
    case err != nil:
        return fmt.Errorf("failed to get resource: %w", err)
    }

    // Deletion - use cascade delete (Foreground), skip NotFound
    err = cli.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationForeground))
    switch {
    case k8serr.IsNotFound(err):
        return nil
    case err != nil:
        return fmt.Errorf("failed to delete resource: %w", err)
    }
    return nil
}
```

## Coding Style

- Use early returns, avoid nested `if` statements
- Use switch case for error handling from k8s API calls
- Always wrap errors with context using `fmt.Errorf("message: %w", err)`
- Handle errors immediately with guard clauses
- Use explicit types in function signatures

**Good:**
```go
err := cli.List(ctx, list)
switch {
case k8serr.IsNotFound(err), meta.IsNoMatchError(err):
    return nil
case err != nil:
    return fmt.Errorf("failed to list resources: %w", err)
case len(list.Items) == 0:
    return nil
}
```

**Avoid:**
```go
err := cli.List(ctx, list)
if err == nil {
    if len(list.Items) > 0 {
        // nested logic
    }
} else if !meta.IsNoMatchError(err) {
    return err // BAD: unwrapped error
}
```

## Testing

Use vanilla Gomega with `NewWithT(t)` and `fakeclient.New()`:

```go
func TestCleanupDeprecatedVAPB(t *testing.T) {
    g := NewWithT(t)
    ctx := t.Context()

    vapb := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
        ObjectMeta: metav1.ObjectMeta{Name: "kueue-validating-admission-policy-binding"},
    }

    cli, err := fakeclient.New(fakeclient.WithObjects(vapb))
    g.Expect(err).ShouldNot(HaveOccurred())

    err = cleanupDeprecatedVAPB(ctx, cli)
    g.Expect(err).ShouldNot(HaveOccurred())

    err = cli.Get(ctx, client.ObjectKey{Name: vapb.Name}, vapb)
    g.Expect(k8serr.IsNotFound(err)).To(BeTrue())
}
```

## Verification

1. Unit tests pass: `make unit-test`
2. Lint passes: `make lint`
3. E2E tests pass: `make e2e-test`
4. Manual verification:
   - Deploy operator with old version
   - Upgrade to new version
   - Verify component conditions show upgrade progress
   - Verify deprecated resources are cleaned up
   - Verify one component failure doesn't block others
