//go:build !minimal

package jobs

import (
	"context"
	"sync"
	"time"

	observabilitysvc "github.com/MalenkiySolovey/solovey-component-observability-extra/history"
)

const (
	observability30sTicks = 15
	observability1mTicks  = 30
	observability5mTicks  = 150
)

type SamplingJob struct {
	mu                   sync.Mutex
	warning              func(...any)
	sampler              Sampler
	ticks                int
	currentObservability func() observabilitysvc.ObservabilitySample
	currentCore          func() observabilitysvc.CoreSample
	now                  func() time.Time
}

type Sampler = observabilitysvc.Sampler

func NewSamplingJob(sampler Sampler, warning func(...any)) *SamplingJob {
	job := &SamplingJob{sampler: sampler, warning: warning}
	job.currentObservability = sampler.CurrentObservabilitySample
	job.currentCore = sampler.CurrentCoreSample
	job.now = time.Now
	return job
}

func (j *SamplingJob) Run() {
	j.RunContext(context.Background())
}

func (j *SamplingJob) RunContext(ctx context.Context) {
	if ctx == nil || ctx.Err() != nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if ctx.Err() != nil {
		return
	}

	if err := j.sampler.RecordObservabilitySample(observabilitysvc.ObservabilityBucket2s, j.currentObservability()); err != nil {
		j.warn("record observability sample failed:", err)
		return
	}
	if ctx.Err() != nil {
		return
	}
	if err := j.sampler.RecordCoreSample(observabilitysvc.ObservabilityBucket2s, j.currentCore()); err != nil {
		j.warn("record core observability sample failed:", err)
		return
	}
	j.ticks++

	j.aggregateEvery(ctx, observabilitysvc.ObservabilityBucket30s, observability30sTicks)
	j.aggregateEvery(ctx, observabilitysvc.ObservabilityBucket1m, observability1mTicks)
	j.aggregateEvery(ctx, observabilitysvc.ObservabilityBucket5m, observability5mTicks)
}

func (j *SamplingJob) aggregateEvery(ctx context.Context, bucket observabilitysvc.ObservabilityBucket, interval int) {
	if ctx.Err() != nil || interval <= 0 || j.ticks%interval != 0 {
		return
	}
	samples, err := j.sampler.HistoryForBucket(observabilitysvc.ObservabilityBucket2s)
	if err != nil {
		j.warn("read observability samples for aggregation failed:", err)
		return
	}
	if len(samples) == 0 {
		return
	}
	if len(samples) > interval {
		samples = samples[len(samples)-interval:]
	}
	ts := j.now().Unix()
	if err := j.sampler.RecordObservabilitySample(bucket, observabilitysvc.AggregateObservabilitySamples(samples, ts)); err != nil {
		j.warn("record aggregated observability sample failed:", err)
	}
	if ctx.Err() != nil {
		return
	}

	coreSamples, err := j.sampler.CoreHistoryForBucket(observabilitysvc.ObservabilityBucket2s)
	if err != nil {
		j.warn("read core samples for aggregation failed:", err)
		return
	}
	if len(coreSamples) == 0 {
		return
	}
	if len(coreSamples) > interval {
		coreSamples = coreSamples[len(coreSamples)-interval:]
	}
	if err := j.sampler.RecordCoreSample(bucket, observabilitysvc.AggregateCoreSamples(coreSamples, ts)); err != nil {
		j.warn("record aggregated core sample failed:", err)
	}
}

func (j *SamplingJob) warn(args ...any) {
	if j.warning != nil {
		j.warning(args...)
	}
}
