package provision_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
)

type readinessStub struct {
	ready map[string]bool
}

func (s *readinessStub) IsReady(_ context.Context, name string) (bool, error) {
	r, ok := s.ready[name]
	if !ok {
		return false, fmt.Errorf("unknown node %q: %w", name, dag.ErrUnknownNode)
	}
	return r, nil
}

type conditionRecorder struct {
	conditions []common.Condition
}

func (c *conditionRecorder) SetCondition(cond common.Condition) {
	c.conditions = append(c.conditions, cond)
}

func (c *conditionRecorder) last() common.Condition {
	return c.conditions[len(c.conditions)-1]
}

func (c *conditionRecorder) byReason(reason string) []common.Condition {
	var result []common.Condition
	for _, cond := range c.conditions {
		if cond.Reason == reason {
			result = append(result, cond)
		}
	}
	return result
}

func resetDefaultRegistry(t *testing.T, entries map[string]dag.Runlevel) {
	t.Helper()
	r := provision.DefaultRegistry()
	r.Reset()
	t.Cleanup(func() { r.Reset() })
	for name, rl := range entries {
		r.Add(name, provision.KindComponent, rl)
	}
}

func TestWalkBatches_AllReady_NoGating(t *testing.T) {
	resetDefaultRegistry(t, map[string]dag.Runlevel{
		"alpha": dag.RL(20),
		"beta":  dag.RL(31),
	})

	checker := &readinessStub{ready: map[string]bool{"alpha": true, "beta": true}}
	tracker := dag.NewStuckTracker()
	conds := &conditionRecorder{}
	var processed []string

	requeueAfter, err := provision.WalkBatches(context.Background(), checker, tracker, "test", conds, func(batch []provision.UnifiedNode) error {
		for _, n := range batch {
			processed = append(processed, n.GetName())
		}
		return nil
	})

	require.NoError(t, err)
	assert.Zero(t, requeueAfter)
	assert.Equal(t, []string{"alpha", "beta"}, processed)
	assert.Equal(t, status.ConditionTypeProvisioningProgress, conds.last().Type)
	assert.Equal(t, "True", string(conds.last().Status))
}

func TestWalkBatches_GatingBlocks(t *testing.T) {
	resetDefaultRegistry(t, map[string]dag.Runlevel{
		"alpha": dag.RL(20),
		"beta":  dag.RL(31),
	})

	checker := &readinessStub{ready: map[string]bool{"alpha": false, "beta": true}}
	tracker := dag.NewStuckTracker()
	conds := &conditionRecorder{}
	var processed []string

	requeueAfter, err := provision.WalkBatches(context.Background(), checker, tracker, "test", conds, func(batch []provision.UnifiedNode) error {
		for _, n := range batch {
			processed = append(processed, n.GetName())
		}
		return nil
	})

	require.NoError(t, err)
	assert.Positive(t, requeueAfter, "should request requeue for remaining timeout")
	assert.Equal(t, []string{"alpha"}, processed, "beta should be gated")
	assert.Equal(t, status.AwaitingReadinessReason, conds.last().Reason)
}

func TestWalkBatches_GatingBlocks_RequeueDurationMatchesRemainingTimeout(t *testing.T) {
	resetDefaultRegistry(t, map[string]dag.Runlevel{
		"alpha": dag.RL(20),
		"beta":  dag.RL(31),
	})

	customTimeout := 5 * time.Minute
	dag.SetRunlevelPolicy(31, dag.RunlevelPolicy{Timeout: customTimeout})
	defer dag.ClearRunlevelPolicy(31)

	checker := &readinessStub{ready: map[string]bool{"alpha": false, "beta": true}}
	tracker := dag.NewStuckTracker()
	conds := &conditionRecorder{}

	requeueAfter, err := provision.WalkBatches(context.Background(), checker, tracker, "test", conds, func(batch []provision.UnifiedNode) error {
		return nil
	})

	require.NoError(t, err)
	assert.InDelta(t, customTimeout.Seconds(), requeueAfter.Seconds(), 1.0,
		"requeue duration should be close to the full timeout on first observation")
	assert.Equal(t, status.AwaitingReadinessReason, conds.last().Reason)
}

func TestWalkBatches_TimeoutAdvancesPastStuckRunlevel(t *testing.T) {
	resetDefaultRegistry(t, map[string]dag.Runlevel{
		"alpha": dag.RL(20),
		"beta":  dag.RL(31),
	})

	dag.SetRunlevelPolicy(31, dag.RunlevelPolicy{Timeout: 1 * time.Millisecond})
	defer dag.ClearRunlevelPolicy(31)

	checker := &readinessStub{ready: map[string]bool{"alpha": false, "beta": true}}
	tracker := dag.NewStuckTracker()
	conds := &conditionRecorder{}

	tracker.Since("test", 31)
	time.Sleep(2 * time.Millisecond)

	var processed []string
	requeueAfter, err := provision.WalkBatches(context.Background(), checker, tracker, "test", conds, func(batch []provision.UnifiedNode) error {
		for _, n := range batch {
			processed = append(processed, n.GetName())
		}
		return nil
	})

	require.NoError(t, err)
	assert.Zero(t, requeueAfter, "timeout already fired, no requeue needed")
	assert.Equal(t, []string{"alpha", "beta"}, processed, "beta should be processed after timeout")
	skipped := conds.byReason(status.RunlevelTimeoutExceededReason)
	require.Len(t, skipped, 1)
	assert.Contains(t, skipped[0].Message, "alpha")
}

func TestWalkBatches_TimeoutPropagates_NoReGating(t *testing.T) {
	resetDefaultRegistry(t, map[string]dag.Runlevel{
		"dashboard": dag.RL(20),
		"kserve":    dag.RL(31),
		"trustyai":  dag.RL(33),
	})

	dag.SetRunlevelPolicy(31, dag.RunlevelPolicy{Timeout: 1 * time.Millisecond})
	defer dag.ClearRunlevelPolicy(31)

	checker := &readinessStub{ready: map[string]bool{
		"dashboard": false,
		"kserve":    true,
		"trustyai":  true,
	}}
	tracker := dag.NewStuckTracker()
	conds := &conditionRecorder{}

	tracker.Since("test", 31)
	time.Sleep(2 * time.Millisecond)

	var processed []string
	requeueAfter, err := provision.WalkBatches(context.Background(), checker, tracker, "test", conds, func(batch []provision.UnifiedNode) error {
		for _, n := range batch {
			processed = append(processed, n.GetName())
		}
		return nil
	})

	require.NoError(t, err)
	assert.Zero(t, requeueAfter, "timeout already fired, no requeue needed")
	assert.Equal(t, []string{"dashboard", "kserve", "trustyai"}, processed,
		"once dashboard times out at RL31, RL33 should not re-gate on it")
}

func TestWalkBatches_TimeoutDoesNotForgiveNewStuckEntry(t *testing.T) {
	resetDefaultRegistry(t, map[string]dag.Runlevel{
		"alpha":   dag.RL(20),
		"bravo":   dag.RL(31),
		"charlie": dag.RL(33),
	})

	dag.SetRunlevelPolicy(31, dag.RunlevelPolicy{Timeout: 1 * time.Millisecond})
	defer dag.ClearRunlevelPolicy(31)

	checker := &readinessStub{ready: map[string]bool{
		"alpha":   false,
		"bravo":   false,
		"charlie": true,
	}}
	tracker := dag.NewStuckTracker()
	conds := &conditionRecorder{}

	tracker.Since("test", 31)
	time.Sleep(2 * time.Millisecond)

	var processed []string
	requeueAfter, err := provision.WalkBatches(context.Background(), checker, tracker, "test", conds, func(batch []provision.UnifiedNode) error {
		for _, n := range batch {
			processed = append(processed, n.GetName())
		}
		return nil
	})

	require.NoError(t, err)
	assert.Positive(t, requeueAfter, "bravo is newly stuck, should request requeue")
	assert.Equal(t, []string{"alpha", "bravo"}, processed,
		"alpha timed out at RL31, but bravo is newly stuck at RL33 and should gate charlie")
	assert.Equal(t, status.AwaitingReadinessReason, conds.last().Reason,
		"should be awaiting readiness on bravo, not skipped")
}

func TestWalkBatches_ProcessBatchErrorHaltsWalk(t *testing.T) {
	resetDefaultRegistry(t, map[string]dag.Runlevel{
		"alpha": dag.RL(20),
		"beta":  dag.RL(31),
	})

	checker := &readinessStub{ready: map[string]bool{"alpha": true, "beta": true}}
	tracker := dag.NewStuckTracker()
	conds := &conditionRecorder{}

	_, err := provision.WalkBatches(context.Background(), checker, tracker, "test", conds, func(batch []provision.UnifiedNode) error {
		return errors.New("reconcile failed")
	})

	require.ErrorContains(t, err, "reconcile failed")
}

// --- Metric tests ---

func resetMetrics() {
	provision.RunlevelStatus.Reset()
	provision.RunlevelDurationSeconds.Reset()
	provision.RunlevelCleared.Set(0)
	provision.RunlevelBlocked.Set(0)
	provision.RunlevelTimeoutTotal.Reset()
}

func rlStatus(runlevel string, status string) float64 {
	return testutil.ToFloat64(provision.RunlevelStatus.WithLabelValues(runlevel, status))
}

func rlDuration(runlevel string) float64 {
	return testutil.ToFloat64(provision.RunlevelDurationSeconds.WithLabelValues(runlevel))
}

func TestWalkBatches_Metrics_AllReady(t *testing.T) {
	resetDefaultRegistry(t, map[string]dag.Runlevel{
		"alpha": dag.RL(20),
		"beta":  dag.RL(31),
	})
	resetMetrics()

	checker := &readinessStub{ready: map[string]bool{"alpha": true, "beta": true}}
	tracker := dag.NewStuckTracker()
	conds := &conditionRecorder{}

	_, err := provision.WalkBatches(context.Background(), checker, tracker, "test", conds, func(batch []provision.UnifiedNode) error {
		return nil
	})
	require.NoError(t, err)

	// Both runlevels processed.
	assert.Equal(t, float64(1), rlStatus("20", provision.StatusProcessed))
	assert.Equal(t, float64(0), rlStatus("20", provision.StatusPending))
	assert.Equal(t, float64(0), rlStatus("20", provision.StatusBlocked))
	assert.Equal(t, float64(0), rlStatus("20", provision.StatusTimedOut))

	assert.Equal(t, float64(1), rlStatus("31", provision.StatusProcessed))
	assert.Equal(t, float64(0), rlStatus("31", provision.StatusBlocked))

	// Aggregate: cleared = 31, blocked = 0.
	assert.Equal(t, float64(31), testutil.ToFloat64(provision.RunlevelCleared))
	assert.Equal(t, float64(0), testutil.ToFloat64(provision.RunlevelBlocked))

	// Two batches processed.
	assert.GreaterOrEqual(t, testutil.ToFloat64(provision.BatchesProcessedTotal), float64(2))

	// Duration >= 0 for both.
	assert.GreaterOrEqual(t, rlDuration("20"), float64(0))
	assert.GreaterOrEqual(t, rlDuration("31"), float64(0))
}

func TestWalkBatches_Metrics_GatingBlocks(t *testing.T) {
	resetDefaultRegistry(t, map[string]dag.Runlevel{
		"alpha": dag.RL(20),
		"beta":  dag.RL(31),
	})
	resetMetrics()

	checker := &readinessStub{ready: map[string]bool{"alpha": false, "beta": true}}
	tracker := dag.NewStuckTracker()
	conds := &conditionRecorder{}

	_, err := provision.WalkBatches(context.Background(), checker, tracker, "test", conds, func(batch []provision.UnifiedNode) error {
		return nil
	})
	require.NoError(t, err)

	// RL20 processed (first batch, never gated).
	assert.Equal(t, float64(1), rlStatus("20", provision.StatusProcessed))

	// RL31 blocked (alpha not ready).
	assert.Equal(t, float64(1), rlStatus("31", provision.StatusBlocked))
	assert.Equal(t, float64(0), rlStatus("31", provision.StatusProcessed))
	assert.Equal(t, float64(0), rlStatus("31", provision.StatusPending))

	// Aggregate: cleared = 20, blocked = 31.
	assert.Equal(t, float64(20), testutil.ToFloat64(provision.RunlevelCleared))
	assert.Equal(t, float64(31), testutil.ToFloat64(provision.RunlevelBlocked))

	// Duration > 0 for blocked runlevel.
	assert.GreaterOrEqual(t, rlDuration("31"), float64(0))
}

func TestWalkBatches_Metrics_Timeout(t *testing.T) {
	resetDefaultRegistry(t, map[string]dag.Runlevel{
		"alpha": dag.RL(20),
		"beta":  dag.RL(31),
	})
	resetMetrics()

	dag.SetRunlevelPolicy(31, dag.RunlevelPolicy{Timeout: 1 * time.Millisecond})
	defer dag.ClearRunlevelPolicy(31)

	checker := &readinessStub{ready: map[string]bool{"alpha": false, "beta": true}}
	tracker := dag.NewStuckTracker()
	conds := &conditionRecorder{}

	tracker.Since("test", 31)
	time.Sleep(2 * time.Millisecond)

	_, err := provision.WalkBatches(context.Background(), checker, tracker, "test", conds, func(batch []provision.UnifiedNode) error {
		return nil
	})
	require.NoError(t, err)

	// RL20 processed (first batch).
	assert.Equal(t, float64(1), rlStatus("20", provision.StatusProcessed))

	// RL31 timed out then processed (timeout advances, then batch runs).
	// The final status for RL31 is "processed" because after timeout the
	// batch is still executed.
	assert.Equal(t, float64(1), rlStatus("31", provision.StatusProcessed))

	// Timeout counter incremented for RL31.
	assert.Equal(t, float64(1),
		testutil.ToFloat64(provision.RunlevelTimeoutTotal.WithLabelValues("31")))

	// Both cleared.
	assert.Equal(t, float64(31), testutil.ToFloat64(provision.RunlevelCleared))
	assert.Equal(t, float64(0), testutil.ToFloat64(provision.RunlevelBlocked))
}
