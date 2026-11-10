//go:build !minimal

package observabilityextra

import (
	"context"

	"errors"
	hostports "github.com/MalenkiySolovey/solovey-host/componenthost"
	"sync"

	"github.com/MalenkiySolovey/solovey-component-observability-extra/jobs"

	"github.com/robfig/cron/v3"
)

type Component struct {
	mu        sync.Mutex
	started   bool
	scheduler hostports.Scheduler
	entryID   cron.EntryID
	warning   func(...any)
}

func (c *Component) Start(_ context.Context, ports hostports.Ports) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return nil
	}
	if ports.Scheduler == nil {
		return errors.New("observability-extra scheduler is unavailable")
	}
	if ports.ObservationSampler == nil {
		return errors.New("observability-extra sampling owner is unavailable")
	}
	entryID, err := ports.Scheduler.AddJob("@every 2s", jobs.NewSamplingJob(ports.ObservationSampler, c.warning))
	if err != nil {
		return err
	}
	c.started = true
	c.scheduler = ports.Scheduler
	c.entryID = entryID
	return nil
}

func (c *Component) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	scheduler := c.scheduler
	entryID := c.entryID
	if scheduler != nil && entryID != 0 {
		if err := scheduler.RemoveJobAndWait(ctx, entryID); err != nil {
			return err
		}
	}
	c.started = false
	c.scheduler = nil
	c.entryID = 0
	return nil
}

// New creates an inactive lifecycle. The warning callback retains host logging ownership.
func New(warning func(...any)) *Component { return &Component{warning: warning} }
