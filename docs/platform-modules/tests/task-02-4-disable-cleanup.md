---
task: "02-4 - TestPlatformOnly_DisableModule_Cleanup"
file: tests/integration/platform/platform_only_test.go
depends_on: task-02-1-two-modules-created.md
started:
completed:
---

# Task 02-4: TestPlatformOnly_DisableModule_Cleanup

Add `TestPlatformOnly_DisableModule_Cleanup` to
`tests/integration/platform/platform_only_test.go`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**: `suite_test.go` provides `startAllControllers`, `suiteOpts`,
`createPlatform`, `createGatewayConfig`.

## Test: `TestPlatformOnly_DisableModule_Cleanup`

Verifies that disabling a module on the Platform CR causes its PlatformModule CR
to be deleted.

**Setup**:
- Empty registries. Call `startAllControllers`.
- Call `createGatewayConfig(t, tc)`.

**Action**:
1. Create Platform with `Monitoring=Managed` and `AIGateway=Managed`.
2. Wait for both PlatformModule CRs to exist.
3. Update Platform to set `Monitoring=Removed` (or empty `ManagementSpec{}`).

**Assertions**:
- PlatformModule `"monitoring"` is deleted (get returns `not found`).
- PlatformModule `"aigateway"` still exists.

Use `g.Eventually(func() error { return cli.Get(...) }).Should(MatchError(ContainSubstring("not found")))`
for the deletion check (same pattern as
`internal/controller/platform/platform_controller_test.go:147-149`).

## Verification

```bash
go test -v -count=1 -run 'TestPlatformOnly_DisableModule_Cleanup' ./tests/integration/platform/...
```
