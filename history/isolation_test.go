//go:build !minimal

package observability

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestIndependentHostsAndResetIsolation(t *testing.T) {
	a := &Service{Store: NewStore(), Settings: &fixedSettings{capMB: 32}}
	b := &Service{Store: NewStore(), Settings: &fixedSettings{capMB: 1}}
	for i := 0; i < 500; i++ {
		_ = a.RecordObservabilitySample(ObservabilityBucket2s, testSample(i))
		_ = b.RecordObservabilitySample(ObservabilityBucket2s, testSample(1000+i))
	}
	a.Store.Reset()
	x, _ := a.HistoryForBucket(ObservabilityBucket2s)
	y, _ := b.HistoryForBucket(ObservabilityBucket2s)
	if len(x) != 0 || len(y) != capsForObservabilityMemory(1)[ObservabilityBucket2s] || y[0].DateTime < 1000 {
		t.Fatal("host stores or reset crossed scope")
	}
}
func TestConcurrentAppendReadAndReset(t *testing.T) {
	s := &Service{Store: NewStore(), Settings: &fixedSettings{capMB: 32}}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			for j := 0; j < 1000; j++ {
				_ = s.RecordObservabilitySample(ObservabilityBucket2s, testSample(j))
				_, _ = s.MetricHistory(ObservabilityMetricCPU, ObservabilityBucket2s, 500)
				if j%100 == 0 {
					s.Store.Reset()
				}
			}
		})
	}
	wg.Wait()
	x, _ := s.HistoryForBucket(ObservabilityBucket2s)
	if len(x) > 300 {
		t.Fatal(len(x))
	}
}
func TestRollupKeepsNumericMeanAndLatestCore(t *testing.T) {
	samples := []ObservabilitySample{{DateTime: 1, CPU: 10, Memory: map[string]interface{}{"a": int32(3), "text": "ignore"}, Network: map[string]interface{}{"recv": uint64(100)}}, {DateTime: 2, CPU: 30, Memory: map[string]interface{}{"a": float32(7), "b": int64(20)}, Network: map[string]interface{}{"recv": uint(300)}}}
	got := AggregateObservabilitySamples(samples, 9)
	want := ObservabilitySample{DateTime: 9, CPU: 20, Memory: map[string]interface{}{"a": float64(5), "b": float64(20)}, Network: map[string]interface{}{"recv": float64(200)}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%#v", got)
	}
	core := AggregateCoreSamples([]CoreSample{{DateTime: 1, Core: map[string]interface{}{"state": "old"}}, {DateTime: 2, Core: map[string]interface{}{"state": "current"}}}, 9)
	if core.DateTime != 9 || core.Core["state"] != "current" {
		t.Fatal(core)
	}
}

type failedSettings struct{}

func (failedSettings) GetObservabilityMemoryCapMB() (int, error) { return 0, errors.New("unavailable") }
func TestInvalidQueriesAndSettingsFallback(t *testing.T) {
	s := &Service{Store: NewStore(), Settings: failedSettings{}}
	_ = s.RecordObservabilitySample(ObservabilityBucket2s, testSample(1))
	if s.Store.history.lastAppliedMemoryCap != 32 {
		t.Fatal("default changed")
	}
	if _, e := s.HistoryForBucket("bad"); e == nil || e.Error() != "invalid observability bucket" {
		t.Fatal(e)
	}
	if _, e := s.MetricHistory("bad", ObservabilityBucket2s, 0); e == nil || e.Error() != "invalid observability metric" {
		t.Fatal(e)
	}
	if b, e := ParseObservabilityBucket(""); e != nil || b != ObservabilityBucket2s {
		t.Fatal(b, e)
	}
}
