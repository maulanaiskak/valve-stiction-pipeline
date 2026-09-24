package main

import (
	"context"
	"sync"
	"testing"

	"google.golang.org/grpc"

	pb "valve-stiction-pipeline/ingestion/detectionpb"
)

// fakeDetector records every window it receives instead of making a real
// gRPC call, so the sliding-window buffering logic can be tested without a
// running detection service.
type fakeDetector struct {
	mu       sync.Mutex
	requests []*pb.WindowRequest
}

func (f *fakeDetector) DetectWindow(_ context.Context, in *pb.WindowRequest, _ ...grpc.CallOption) (*pb.WindowResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, in)
	return &pb.WindowResponse{Label: "no"}, nil
}

func (f *fakeDetector) received() []*pb.WindowRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*pb.WindowRequest, len(f.requests))
	copy(out, f.requests)
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

	detector := &fakeDetector{}
	in := newIngestor(detector)

	feedSamples(in, "valve-1", 250, 1000)

	reqs := detector.received()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 non-overlapping windows from 250 samples, got %d", len(reqs))
	}
	if len(reqs[0].Pv) != WindowSize || len(reqs[1].Pv) != WindowSize {
		t.Fatalf("expected windows of size %d, got %d and %d", WindowSize, len(reqs[0].Pv), len(reqs[1].Pv))
	}
	// second window should start where the first ended (sample index 100),
	// timestamp 1000 + 100*100 = 11000
	if reqs[1].WindowStartUnixMs != 1000+100*100 {
		t.Fatalf("expected second window to start at ts=11000, got %d", reqs[1].WindowStartUnixMs)
	}
	// non-overlapping: first window's last PV value (99) should not reappear
	// in the second window's first value (should be 100)
	if reqs[1].Pv[0] != 100 {
		t.Fatalf("expected second window to start at PV=100 (no overlap), got %v", reqs[1].Pv[0])
	}
}

func TestSlidingWindows_SmallerStride(t *testing.T) {
	origStride := WindowStride
	WindowStride = 10
	defer func() { WindowStride = origStride }()

	detector := &fakeDetector{}
	in := newIngestor(detector)

	feedSamples(in, "valve-1", 150, 1000)

	reqs := detector.received()
	// first window ready at sample 100, then every 10 samples after: 100,110,...,150 -> 6 windows
	if len(reqs) != 6 {
		t.Fatalf("expected 6 overlapping windows from 150 samples at stride 10, got %d", len(reqs))
	}
	for i, r := range reqs {
		if len(r.Pv) != WindowSize {
			t.Fatalf("window %d: expected size %d, got %d", i, WindowSize, len(r.Pv))
		}
	}
	// consecutive windows should overlap: second window's PV should start
	// exactly WindowStride samples after the first
	if reqs[1].Pv[0]-reqs[0].Pv[0] != float64(WindowStride) {
		t.Fatalf("expected consecutive windows to be offset by stride=%d, got offset %v",
			WindowStride, reqs[1].Pv[0]-reqs[0].Pv[0])
	}
}

func TestSensorsAreBufferedIndependently(t *testing.T) {
	origStride := WindowStride
	WindowStride = WindowSize
	defer func() { WindowStride = origStride }()

	detector := &fakeDetector{}
	in := newIngestor(detector)

	feedSamples(in, "valve-1", 100, 1000)
	feedSamples(in, "valve-2", 50, 2000) // not enough for a window yet

	reqs := detector.received()
	if len(reqs) != 1 {
		t.Fatalf("expected only valve-1's window to be ready, got %d windows", len(reqs))
	}
	if reqs[0].SensorId != "valve-1" {
		t.Fatalf("expected valve-1's window, got sensor_id=%s", reqs[0].SensorId)
	}
}
