package controller

import (
	"math"
	"sort"
	"sync"
	"time"

	"sound-signal-generator/internal/audio"
	"sound-signal-generator/internal/engine"
)

// DiagnosticsReport is returned to the UI at 10Hz polling frequency.
type DiagnosticsReport struct {
	State                     engine.State   `json:"state"`
	DeviceName                string         `json:"device_name"`
	Exclusive                 bool           `json:"exclusive"`
	SampleRate                int            `json:"sample_rate"`
	SampleFormat              string         `json:"sample_format"`
	Channels                  int            `json:"channels"`
	BufferPeriodMs            float64        `json:"buffer_period_ms"`
	BufferFrames              int            `json:"buffer_frames"`
	BitPerfectStatus          BitPerfectTier `json:"bit_perfect_status"`
	RenderCallbackCount       uint64         `json:"render_callback_count"`
	UnderrunCount             uint64         `json:"underrun_count"`
	LateCallbackCount         uint64         `json:"late_callback_count"`
	LastRenderTimeUs          float64        `json:"last_render_time_us"`
	MaxRenderTimeUs           float64        `json:"max_render_time_us"`
	P999RenderTimeUs          float64        `json:"p999_render_time_us"`
	RenderTimePercentOfBuffer float64        `json:"render_time_percent_of_buffer"`
	MaxPercentOfBuffer        float64        `json:"max_percent_of_buffer"`
	P999PercentOfBuffer       float64        `json:"p999_percent_of_buffer"`
	Timestamp                 time.Time      `json:"timestamp"`
}

// PercentileTracker maintains a circular window of recent render durations.
type PercentileTracker struct {
	mu      sync.Mutex
	samples []int64 // durations in nanoseconds
	idx     int
	count   int
	maxSize int
}

// NewPercentileTracker creates a tracker with given ring buffer capacity.
func NewPercentileTracker(size int) *PercentileTracker {
	if size <= 0 {
		size = 10000
	}
	return &PercentileTracker{
		samples: make([]int64, size),
		maxSize: size,
	}
}

// Record appends a new duration sample in nanoseconds.
func (pt *PercentileTracker) Record(durationNs int64) {
	pt.mu.Lock()
	pt.samples[pt.idx] = durationNs
	pt.idx = (pt.idx + 1) % pt.maxSize
	if pt.count < pt.maxSize {
		pt.count++
	}
	pt.mu.Unlock()
}

// Percentile calculates a percentile duration in nanoseconds using the
// nearest-rank definition. The percentile is clamped to [0, 1].
func (pt *PercentileTracker) Percentile(percentile float64) int64 {
	if percentile < 0 {
		percentile = 0
	} else if percentile > 1 {
		percentile = 1
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.count == 0 {
		return 0
	}

	cpy := make([]int64, pt.count)
	copy(cpy, pt.samples[:pt.count])
	sort.Slice(cpy, func(i, j int) bool { return cpy[i] < cpy[j] })

	rank := int(math.Ceil(percentile*float64(pt.count))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= pt.count {
		rank = pt.count - 1
	}
	return cpy[rank]
}

// P50, P99, and P999 return common render-duration percentiles.
func (pt *PercentileTracker) P50() int64  { return pt.Percentile(0.50) }
func (pt *PercentileTracker) P99() int64  { return pt.Percentile(0.99) }
func (pt *PercentileTracker) P999() int64 { return pt.Percentile(0.999) }

// Reset clears recorded percentile history.
func (pt *PercentileTracker) Reset() {
	pt.mu.Lock()
	pt.idx = 0
	pt.count = 0
	pt.mu.Unlock()
}

// DiagnosticsService collects engine metrics and produces 10Hz reports.
type DiagnosticsService struct {
	tracker *PercentileTracker
}

// NewDiagnosticsService creates a diagnostics service.
func NewDiagnosticsService() *DiagnosticsService {
	return &DiagnosticsService{
		tracker: NewPercentileTracker(20000),
	}
}

// Tracker returns the percentile tracker for recording engine render times.
func (s *DiagnosticsService) Tracker() *PercentileTracker {
	return s.tracker
}

// BuildReport constructs an immutable 10Hz diagnostics snapshot.
func (s *DiagnosticsService) BuildReport(
	cStatus ControllerStatus,
	bitTier BitPerfectTier,
	bufferDuration time.Duration,
	exclusive bool,
) DiagnosticsReport {
	diag := cStatus.Diagnostics

	bufferPeriodMs := float64(bufferDuration.Microseconds()) / 1000.0
	if bufferPeriodMs <= 0 {
		bufferPeriodMs = 10.0 // default 10ms
	}

	bufferPeriodNs := float64(bufferDuration.Nanoseconds())
	if bufferPeriodNs <= 0 {
		bufferPeriodNs = 10_000_000.0
	}

	frames := int(float64(cStatus.ActiveFormat.SampleRate) * (bufferPeriodMs / 1000.0))

	lastUs := float64(diag.LastRenderDuration.Nanoseconds()) / 1000.0
	maxUs := float64(diag.MaxRenderDuration.Nanoseconds()) / 1000.0

	// Record to percentile tracker
	if diag.LastRenderDuration > 0 {
		s.tracker.Record(diag.LastRenderDuration.Nanoseconds())
	}

	p999Ns := s.tracker.P999()
	if p999Ns == 0 && diag.LastRenderDuration > 0 {
		p999Ns = diag.LastRenderDuration.Nanoseconds()
	}
	p999Us := float64(p999Ns) / 1000.0

	renderPercent := (float64(diag.LastRenderDuration.Nanoseconds()) / bufferPeriodNs) * 100.0
	maxPercent := (float64(diag.MaxRenderDuration.Nanoseconds()) / bufferPeriodNs) * 100.0
	p999Percent := (float64(p999Ns) / bufferPeriodNs) * 100.0

	var fmtStr string
	switch cStatus.ActiveFormat.SampleFormat {
	case audio.SampleFormat16Int:
		fmtStr = "16-bit Integer"
	case audio.SampleFormat24Packed:
		fmtStr = "24-bit Packed"
	case audio.SampleFormat24In32:
		fmtStr = "24-in-32 LE"
	case audio.SampleFormat32Float:
		fmtStr = "32-bit Float"
	default:
		fmtStr = "Unknown"
	}

	return DiagnosticsReport{
		State:                     cStatus.State,
		DeviceName:                cStatus.DeviceName,
		Exclusive:                 exclusive,
		SampleRate:                cStatus.ActiveFormat.SampleRate,
		SampleFormat:              fmtStr,
		Channels:                  cStatus.ActiveFormat.Channels,
		BufferPeriodMs:            bufferPeriodMs,
		BufferFrames:              frames,
		BitPerfectStatus:          bitTier,
		RenderCallbackCount:       diag.RenderCallbackCount,
		UnderrunCount:             diag.UnderrunCount,
		LateCallbackCount:         diag.LateCallbackCount,
		LastRenderTimeUs:          lastUs,
		MaxRenderTimeUs:           maxUs,
		P999RenderTimeUs:          p999Us,
		RenderTimePercentOfBuffer: renderPercent,
		MaxPercentOfBuffer:        maxPercent,
		P999PercentOfBuffer:       p999Percent,
		Timestamp:                 time.Now().UTC(),
	}
}
