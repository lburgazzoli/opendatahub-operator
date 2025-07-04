package bdd

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

type CapturingT struct {
	mu  sync.Mutex
	err *multierror.Error
}

func (c *CapturingT) Helper() {}

func (c *CapturingT) Fatalf(format string, args ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.err = multierror.Append(c.err, fmt.Errorf(format, args...))
}

func (c *CapturingT) Error() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.err.ErrorOrNil()
}

func (c *CapturingT) NewWithT(cfg *Config) *gomega.WithT {
	g := gomega.NewWithT(c)
	g.DurationBundle.EnforceDefaultTimeoutsWhenUsingContexts = true
	g.DurationBundle.EventuallyTimeout = cfg.Eventually.Timeout
	g.DurationBundle.EventuallyPollingInterval = cfg.Eventually.Interval
	g.DurationBundle.ConsistentlyDuration = cfg.Consistently.Timeout
	g.DurationBundle.ConsistentlyPollingInterval = cfg.Consistently.Interval

	format.MaxLength = 0

	return g
}
