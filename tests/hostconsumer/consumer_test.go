//go:build !minimal

package hostconsumer_test

import (
	"context"
	extra "github.com/MalenkiySolovey/solovey-component-observability-extra"
	history "github.com/MalenkiySolovey/solovey-component-observability-extra/history"
	"github.com/MalenkiySolovey/solovey-host/componenthost"
	"github.com/MalenkiySolovey/solovey-host/hostref"
	"github.com/robfig/cron/v3"
	"testing"
)

type snapshots struct{ tick int }

func (s *snapshots) CurrentObservabilitySample() history.ObservabilitySample {
	s.tick++
	return history.ObservabilitySample{DateTime: int64(s.tick), CPU: float64(s.tick), Memory: map[string]interface{}{"current": uint64(123)}, Network: map[string]interface{}{"sent": uint64(45), "recv": uint64(67)}}
}
func (s *snapshots) CurrentCoreSample() history.CoreSample {
	return history.CoreSample{DateTime: int64(s.tick), Core: map[string]interface{}{"state": "running", "source": "node-local"}}
}

type settings struct{}

func (settings) GetObservabilityMemoryCapMB() (int, error) { return 1, nil }

type scheduler struct {
	job    cron.Job
	active bool
	added  int
}

func (s *scheduler) AddJob(_ string, j cron.Job) (cron.EntryID, error) {
	s.job = j
	s.active = true
	s.added++
	return cron.EntryID(s.added), nil
}
func (*scheduler) Schedule(cron.Schedule, cron.Job) cron.EntryID { return 0 }
func (s *scheduler) RemoveJob(cron.EntryID)                      { s.active = false }
func (s *scheduler) RemoveJobAndWait(context.Context, cron.EntryID) error {
	s.active = false
	return nil
}
func (s *scheduler) tick() {
	if s.active {
		s.job.Run()
	}
}

// This is a headless source-compatibility fixture, without a production Node runtime.
func TestIndependentHostUsesTheSameLifecycleAndHistory(t *testing.T) {
	facts := &snapshots{}
	store := history.NewStore()
	owner := &history.Service{Snapshots: facts, Store: store, Settings: settings{}}
	clock := &scheduler{}
	lifecycle := extra.New(nil)
	ports := componenthost.Ports{Host: hostref.Ref{Kind: "NODE", ID: "fixture-node"}, Scheduler: clock, ObservationSampler: owner}
	// Installed but inactive: constructing the feature and ticking a host scheduler does nothing.
	for i := 0; i < 10; i++ {
		clock.tick()
	}
	if facts.tick != 0 || clock.added != 0 {
		t.Fatal("inactive host sampled")
	}
	if err := lifecycle.Start(context.Background(), ports); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 600; i++ {
		clock.tick()
	}
	h, err := owner.HistoryForBucket(history.ObservabilityBucket2s)
	if err != nil || len(h) == 0 || len(h) >= 300 {
		t.Fatalf("memory bound %d %v", len(h), err)
	}
	core, _ := owner.CoreHistoryForBucket(history.ObservabilityBucket5m)
	rolled, _ := owner.HistoryForBucket(history.ObservabilityBucket5m)
	// At the existing one-MB policy the 2s ring retains 110 samples. Rollup
	// averages the available tail, preserving its behavior under memory pressure.
	if len(rolled) != 4 || rolled[3].CPU != 545.5 {
		t.Fatalf("5m rollup changed: %#v", rolled)
	}
	if len(core) != 4 || core[3].Core["source"] != "node-local" {
		t.Fatal(core)
	}
	if err := lifecycle.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := facts.tick
	for i := 0; i < 100; i++ {
		clock.tick()
	}
	if facts.tick != before || clock.active {
		t.Fatal("stopped host continued sampling")
	}
	other := &history.Service{Store: history.NewStore(), Settings: settings{}}
	empty, _ := other.HistoryForBucket(history.ObservabilityBucket2s)
	if len(empty) != 0 {
		t.Fatal("history escaped host scope")
	}
	// No history value is written to Health or assigned a newer timestamp by a query.
	observed := h[len(h)-1].DateTime
	again, _ := owner.HistoryForBucket(history.ObservabilityBucket2s)
	if again[len(again)-1].DateTime != observed {
		t.Fatal("query promoted stale history to fresh observation")
	}
}
