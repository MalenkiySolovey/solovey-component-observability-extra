//go:build !minimal

package observabilityextra

import (
	"context"
	"errors"
	"github.com/MalenkiySolovey/solovey-host/componenthost"
	"github.com/MalenkiySolovey/solovey-host/observation"
	"github.com/robfig/cron/v3"
	"sync"
	"testing"
)

type trackingScheduler struct {
	added, removed    int
	addErr, removeErr error
	job               cron.Job
}

func (s *trackingScheduler) AddJob(spec string, j cron.Job) (cron.EntryID, error) {
	if spec != "@every 2s" {
		panic(spec)
	}
	if s.addErr != nil {
		return 0, s.addErr
	}
	s.added++
	s.job = j
	return cron.EntryID(s.added), nil
}
func (*trackingScheduler) Schedule(cron.Schedule, cron.Job) cron.EntryID { return 0 }
func (s *trackingScheduler) RemoveJob(cron.EntryID)                      { s.removed++ }
func (s *trackingScheduler) RemoveJobAndWait(context.Context, cron.EntryID) error {
	s.removed++
	return s.removeErr
}

type trackingSampler struct{ observations, cores int }

func (*trackingSampler) CurrentObservabilitySample() observation.ObservabilitySample {
	return observation.ObservabilitySample{}
}
func (*trackingSampler) CurrentCoreSample() observation.CoreSample { return observation.CoreSample{} }
func (s *trackingSampler) RecordObservabilitySample(observation.ObservabilityBucket, observation.ObservabilitySample) error {
	s.observations++
	return nil
}
func (s *trackingSampler) RecordCoreSample(observation.ObservabilityBucket, observation.CoreSample) error {
	s.cores++
	return nil
}
func (*trackingSampler) HistoryForBucket(observation.ObservabilityBucket) ([]observation.ObservabilitySample, error) {
	return nil, nil
}
func (*trackingSampler) CoreHistoryForBucket(observation.ObservabilityBucket) ([]observation.CoreSample, error) {
	return nil, nil
}

func TestStopFailureRetainsStateForRetry(t *testing.T) {
	s := &trackingScheduler{removeErr: errors.New("job still running")}
	c := New(nil)
	ports := componenthost.Ports{Scheduler: s, ObservationSampler: &trackingSampler{}}
	if err := c.Start(context.Background(), ports); err != nil {
		t.Fatal(err)
	}
	if c.Stop(context.Background()) == nil || !c.started || c.scheduler != s || c.entryID == 0 {
		t.Fatal("stop failure discarded retry state")
	}
	if err := c.Start(context.Background(), ports); err != nil || s.added != 1 {
		t.Fatal("uncertain stop allowed duplicate job")
	}
	s.removeErr = nil
	if err := c.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.started || c.scheduler != nil || c.entryID != 0 {
		t.Fatal("successful stop retained active state")
	}
}
func TestConcurrentStartAndRepeatedStop(t *testing.T) {
	s := &trackingScheduler{}
	c := New(nil)
	ports := componenthost.Ports{Scheduler: s, ObservationSampler: &trackingSampler{}}
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Go(func() {
			if err := c.Start(context.Background(), ports); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if s.added != 1 {
		t.Fatal(s.added)
	}
	for i := 0; i < 40; i++ {
		wg.Go(func() {
			if err := c.Stop(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if s.removed != 1 {
		t.Fatal(s.removed)
	}
	if err := c.Start(context.Background(), ports); err != nil {
		t.Fatal(err)
	}
	if s.added != 2 {
		t.Fatal("restart did not schedule exactly once")
	}
	_ = c.Stop(context.Background())
}
func TestMissingCapabilitiesAndFailedRegistrationLeaveNoWork(t *testing.T) {
	for _, ports := range []componenthost.Ports{{}, {Scheduler: &trackingScheduler{}}, {ObservationSampler: &trackingSampler{}}, {Scheduler: &trackingScheduler{addErr: errors.New("schedule failed")}, ObservationSampler: &trackingSampler{}}} {
		c := New(nil)
		if c.Start(context.Background(), ports) == nil || c.started || c.entryID != 0 {
			t.Fatal("missing capability admitted")
		}
		if err := c.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}
func TestConstructionIsInactiveAndCanceledSamplingDoesNoWork(t *testing.T) {
	s := &trackingScheduler{}
	sampler := &trackingSampler{}
	c := New(nil)
	if s.added != 0 || sampler.observations != 0 || c.started {
		t.Fatal("construction activated work")
	}
	_ = c.Start(context.Background(), componenthost.Ports{Scheduler: s, ObservationSampler: sampler})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.job.(interface{ RunContext(context.Context) }).RunContext(ctx)
	if sampler.observations != 0 || sampler.cores != 0 {
		t.Fatal("canceled sample read facts")
	}
	_ = c.Stop(context.Background())
}
