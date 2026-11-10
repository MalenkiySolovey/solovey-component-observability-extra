//go:build !minimal

package observability

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
)

func TestHistoryBehaviorDigest(t *testing.T) {
	resetStore(t)
	s := &Service{Store: NewStore(), Settings: &fixedSettings{capMB: 32}}
	for i := 0; i < 1600; i++ {
		for _, bucket := range []ObservabilityBucket{ObservabilityBucket2s, ObservabilityBucket30s, ObservabilityBucket1m, ObservabilityBucket5m} {
			_ = s.RecordObservabilitySample(bucket, resourceSample(i))
			_ = s.RecordCoreSample(bucket, CoreSample{DateTime: int64(i), Core: map[string]interface{}{"state": "running", "tick": i}})
		}
	}
	var result []interface{}
	for _, bucket := range []ObservabilityBucket{ObservabilityBucket2s, ObservabilityBucket30s, ObservabilityBucket1m, ObservabilityBucket5m} {
		h, _ := s.HistoryForBucketSince(bucket, 1400)
		c, _ := s.CoreHistoryForBucketSince(bucket, 1400)
		result = append(result, h, c, AggregateObservabilitySamples(h, 1700), AggregateCoreSamples(c, 1700))
		for _, metric := range []ObservabilityMetric{ObservabilityMetricCPU, ObservabilityMetricRAM, ObservabilityMetricNetIn, ObservabilityMetricNetOut} {
			m, _ := s.MetricHistory(metric, bucket, 1500)
			result = append(result, m)
		}
	}
	b, _ := json.Marshal(result)
	t.Logf("history_behavior_sha256=%x bytes=%d", sha256.Sum256(b), len(b))
	if fmt.Sprintf("%x", sha256.Sum256(b)) != "849f432a5e4592291cf39944f6df446ad3f4bfc8c073403271027f0ef0ccea7b" {
		t.Fatal("baseline history output changed")
	}
}

func resourceSample(i int) ObservabilitySample {
	return ObservabilitySample{DateTime: int64(i), CPU: float64(i % 100), Memory: map[string]interface{}{"current": uint64(i), "total": uint64(1024)}, Network: map[string]interface{}{"recv": uint64(i * 2), "sent": uint64(i * 3)}}
}

func BenchmarkSampleAppend(b *testing.B) {
	resetStore(b)
	s := &Service{Store: NewStore(), Settings: &fixedSettings{capMB: 32}}
	sample := resourceSample(1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.RecordObservabilitySample(ObservabilityBucket2s, sample)
		_ = s.RecordCoreSample(ObservabilityBucket2s, CoreSample{DateTime: 1})
	}
}

func BenchmarkBucketRollup(b *testing.B) {
	samples := make([]ObservabilitySample, 150)
	for i := range samples {
		samples[i] = resourceSample(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = AggregateObservabilitySamples(samples, 1700)
	}
}

func TestHistoryRetainedMemoryPlateau(t *testing.T) {
	resetStore(t)
	s := &Service{Store: NewStore(), Settings: &fixedSettings{capMB: 32}}
	fill := func(n int) {
		for i := 0; i < n; i++ {
			for _, bucket := range []ObservabilityBucket{ObservabilityBucket2s, ObservabilityBucket30s, ObservabilityBucket1m, ObservabilityBucket5m} {
				_ = s.RecordObservabilitySample(bucket, resourceSample(i))
				_ = s.RecordCoreSample(bucket, CoreSample{DateTime: int64(i), Core: map[string]interface{}{"tick": i}})
			}
		}
	}
	var a, z runtime.MemStats
	fill(2000)
	runtime.GC()
	runtime.ReadMemStats(&a)
	g := runtime.NumGoroutine()
	fill(20000)
	runtime.GC()
	runtime.ReadMemStats(&z)
	for bucket, cap := range observabilityDefaultBucketCaps {
		h, _ := s.HistoryForBucket(bucket)
		c, _ := s.CoreHistoryForBucket(bucket)
		if len(h) != cap || len(c) != cap {
			t.Fatalf("unbounded %s %d %d", bucket, len(h), len(c))
		}
	}
	t.Logf("retained_heap_before=%d after=%d delta=%d goroutines_before=%d after=%d", a.HeapAlloc, z.HeapAlloc, int64(z.HeapAlloc)-int64(a.HeapAlloc), g, runtime.NumGoroutine())
	if int64(z.HeapAlloc)-int64(a.HeapAlloc) > 2*1024*1024 {
		t.Fatal("retained history did not plateau")
	}
}
