package main

import (
	"sync"
	"testing"
)

// recordedWindow is what fakePublisher captures per Publish call --
// mirrors windowPublisher.Publish's arguments so tests don't need to care
// whether the real implementation is grpcPublisher or kafkaPublisher.
type recordedWindow struct {
	sensorID    string
	pv, op      []float64
	windowStart int64
}

// fakePublisher records every window it receives instead of making a real
// gRPC call or Kafka publish, so the sliding-window buffering logic can be
// tested without a running detection service or broker.
type fakePublisher struct {
	mu      sync.Mutex
	windows []recordedWindow
}

func (f *fakePublisher) Publish(sensorID string, pv, op []float64, windowStart int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.windows = append(f.windows, recordedWindow{sensorID: sensorID, pv: pv, op: op, windowStart: windowStart})
}

func (f *fakePublisher) received() []recordedWindow {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]recordedWindow, len(f.windows))
	copy(out, f.windows)
	return out
}

func feedSamples(in *ingestor, sensorID string, n int, startTS int64) {
	for i := 0; i < n; i++ {
		in.handleSample(Sample{
			SensorID: sensorID,
			PV:       float64(i),
			OP:       float64(i) * 2,
			TSUnixMs: startTS + int64(i)*100,
		})
	}
}

func TestNonOverlappingWindows_DefaultStride(t *testing.T) {
	origStride := WindowStride
	WindowStride = WindowSize
	defer func() { WindowStride = origStride }()

	publisher := &fakePublisher{}
	in := newIngestor(publisher)

	feedSamples(in, "valve-1", 250, 1000)

	windows := publisher.received()
	if len(windows) != 2 {
		t.Fatalf("expected 2 non-overlapping windows from 250 samples, got %d", len(windows))
	}
	if len(windows[0].pv) != WindowSize || len(windows[1].pv) != WindowSize {
		t.Fatalf("expected windows of size %d, got %d and %d", WindowSize, len(windows[0].pv), len(windows[1].pv))
	}
	// second window should start where the first ended (sample index 100),
	// timestamp 1000 + 100*100 = 11000
	if windows[1].windowStart != 1000+100*100 {
		t.Fatalf("expected second window to start at ts=11000, got %d", windows[1].windowStart)
	}
	// non-overlapping: first window's last PV value (99) should not reappear
	// in the second window's first value (should be 100)
	if windows[1].pv[0] != 100 {
		t.Fatalf("expected second window to start at PV=100 (no overlap), got %v", windows[1].pv[0])
	}
}

func TestSlidingWindows_SmallerStride(t *testing.T) {
	origStride := WindowStride
	WindowStride = 10
	defer func() { WindowStride = origStride }()

	publisher := &fakePublisher{}
	in := newIngestor(publisher)

	feedSamples(in, "valve-1", 150, 1000)

	windows := publisher.received()
	// first window ready at sample 100, then every 10 samples after: 100,110,...,150 -> 6 windows
	if len(windows) != 6 {
		t.Fatalf("expected 6 overlapping windows from 150 samples at stride 10, got %d", len(windows))
	}
	for i, w := range windows {
		if len(w.pv) != WindowSize {
			t.Fatalf("window %d: expected size %d, got %d", i, WindowSize, len(w.pv))
		}
	}
	// consecutive windows should overlap: second window's PV should start
	// exactly WindowStride samples after the first
	if windows[1].pv[0]-windows[0].pv[0] != float64(WindowStride) {
		t.Fatalf("expected consecutive windows to be offset by stride=%d, got offset %v",
			WindowStride, windows[1].pv[0]-windows[0].pv[0])
	}
}

func TestSensorsAreBufferedIndependently(t *testing.T) {
	origStride := WindowStride
	WindowStride = WindowSize
	defer func() { WindowStride = origStride }()

	publisher := &fakePublisher{}
	in := newIngestor(publisher)

	feedSamples(in, "valve-1", 100, 1000)
	feedSamples(in, "valve-2", 50, 2000) // not enough for a window yet

	windows := publisher.received()
	if len(windows) != 1 {
		t.Fatalf("expected only valve-1's window to be ready, got %d windows", len(windows))
	}
	if windows[0].sensorID != "valve-1" {
		t.Fatalf("expected valve-1's window, got sensor_id=%s", windows[0].sensorID)
	}
}
