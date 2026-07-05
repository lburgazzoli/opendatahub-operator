---
task: "05-5 - TestCombined_DAG_DSCIModuleGatesDSCModule"
file: tests/integration/platform/platform_combined_test.go
depends_on: task-05-4-combined-modules.md
started:
completed:
---

# Task 05-5: TestCombined_DAG_DSCIModuleGatesDSCModule

Add `TestCombined_DAG_DSCIModuleGatesDSCModule` to
`tests/integration/platform/platform_combined_test.go`.

## Test: `TestCombined_DAG_DSCIModuleGatesDSCModule`

A DSCI-driven module at RL10 should gate a DSC-driven module at RL20 in the
shared Platform DAG.

**Setup**:
- Monitoring module handler at RL10.
- AIGateway module handler at RL20.
- Shared `moduleReg`, isolated `provisionReg`.
- Register both test CRDs.
- Create `GatewayConfig`, DSCI, and DSC.

**Assertions**:
1. `PlatformModule/monitoring` and `PlatformModule/aigateway` are created.
2. While monitoring is not ready, Platform `ProvisioningProgress=False` and the
   DAG metrics show RL20 blocked.
3. After marking the monitoring operand CR `Ready=True`, RL10 clears and RL20
   proceeds.
4. After marking the aigateway operand CR `Ready=True`, Platform
   `ModulesReady=True` and `Ready=True`.

## Verification

```bash
go test -failfast -timeout 2m -count=1 -run 'TestCombined_DAG_DSCIModuleGatesDSCModule' ./tests/integration/platform/...
```
