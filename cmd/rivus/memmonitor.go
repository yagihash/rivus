package main

import (
	"math"
	"runtime/metrics"
	"sync"
	"time"
)

// What the run did with its memory limit (ADR-121, "実行の終わりの報告"): the most memory that the Go runtime counted in use during
// the run, and the share of the CPU time that went to the collector. Both are only read: nothing here changes what the analysis does
// or finds.

const (
	metricMemTotal    = "/memory/classes/total:bytes"
	metricMemReleased = "/memory/classes/heap/released:bytes"
	metricGCCPU       = "/cpu/classes/gc/total:cpu-seconds"
	metricTotalCPU    = "/cpu/classes/total:cpu-seconds"

	memSampleEvery = 250 * time.Millisecond
)

// memMonitor reads runtime/metrics (not runtime.ReadMemStats, which stops the world) while the analysis runs.
type memMonitor struct {
	samples []metrics.Sample
	gc0     float64
	cpu0    float64

	mu   sync.Mutex
	peak int64

	gc, cpu float64

	quit chan struct{}
	done chan struct{}
	once sync.Once
}

// memReport is what the monitor says at the end of the run.
type memReport struct {
	// PeakBytes is the most that the runtime counted in use: total memory minus what it gave back, the amount that
	// debug.SetMemoryLimit looks at.
	PeakBytes int64
	// GCCPUFraction is the share of the CPU time of the run (from the start of the monitor) that the collector used, in [0, 1].
	GCCPUFraction float64
}

func startMemMonitor() *memMonitor {
	m := &memMonitor{
		samples: []metrics.Sample{{Name: metricMemTotal}, {Name: metricMemReleased}, {Name: metricGCCPU}, {Name: metricTotalCPU}},
		quit:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	_, m.gc0, m.cpu0 = m.read()
	m.sample()
	go func() {
		defer close(m.done)
		t := time.NewTicker(memSampleEvery)
		defer t.Stop()
		for {
			select {
			case <-m.quit:
				return
			case <-t.C:
				m.sample()
			}
		}
	}()
	return m
}

// read gives the memory in use and the CPU seconds of the collector and in all. Only the goroutine of the monitor and
// start/stop (never at the same time) use the samples.
func (m *memMonitor) read() (inUse int64, gcCPU, totalCPU float64) {
	metrics.Read(m.samples)
	u := func(s metrics.Sample) uint64 {
		if s.Value.Kind() == metrics.KindUint64 {
			return s.Value.Uint64()
		}
		return 0
	}
	f := func(s metrics.Sample) float64 {
		if s.Value.Kind() == metrics.KindFloat64 {
			return s.Value.Float64()
		}
		return 0
	}
	total, released := u(m.samples[0]), u(m.samples[1])
	if released > total {
		released = total
	}
	return int64(total - released), f(m.samples[2]), f(m.samples[3])
}

func (m *memMonitor) sample() {
	n, _, _ := m.read()
	m.mu.Lock()
	if n > m.peak {
		m.peak = n
	}
	m.mu.Unlock()
}

// stop ends the goroutine (it leaves none) and gives the report. It may be called more than once.
func (m *memMonitor) stop() memReport {
	m.once.Do(func() {
		close(m.quit)
		<-m.done
		m.sample()
		_, gc, cpu := m.read()
		m.gc, m.cpu = gc-m.gc0, cpu-m.cpu0
	})
	m.mu.Lock()
	defer m.mu.Unlock()
	r := memReport{PeakBytes: m.peak}
	if m.cpu > 0 {
		r.GCCPUFraction = math.Round(math.Min(math.Max(m.gc/m.cpu, 0), 1)*1000) / 1000
	}
	return r
}

// exceedTolerance is how far above the limit the peak has to be before the limit counts as exceeded: 5%. The limit is soft, and when
// what the analysis holds live is lower than the limit, the peak stays close to the limit, a little under or over it (0.5% under, in a
// run of a large target with the default limit); only a limit that is lower than the live memory gives a clearly higher peak (1.47
// times, with a limit that is two thirds of it). A line at the limit itself would give true to a run that is fine, by the jitter of
// the samples.
const exceedTolerance = 0.05

// memoryExceeded says whether the peak is more than 5% above the limit. limit 0 is no limit: never.
func memoryExceeded(peak, limit int64) bool {
	return limit > 0 && float64(peak) > float64(limit)*(1+exceedTolerance)
}
