// Ingestion service (PRD FR-2): subscribes to MQTT, buffers each sensor's
// PV/OP into fixed-size windows, forwards full windows to the detection
// service over gRPC.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "valve-stiction-pipeline/ingestion/detectionpb"
)

// WindowSize matches what valve-stiction-ml validated the classic detector
// against (see its ML_PLAN.md §13) -- not an arbitrary choice, and not
// configurable for that reason: changing it would mean scoring windows the
// classic detector was never validated against.
const WindowSize = 100

// WindowStride controls how far the window slides after each emission.
// Defaults to WindowSize (non-overlapping, matching training exactly).
// A smaller stride gives more frequent detections at the cost of adjacent
// windows sharing samples -- each individual window is still exactly
// WindowSize samples, the same content shape training validated, just
// sampled more often. Configurable via WINDOW_STRIDE since this tradeoff
// (responsiveness vs. redundant detections) has no single right answer.
var WindowStride = WindowSize

type Sample struct {
	SensorID string  `json:"sensor_id"`
	PV       float64 `json:"pv"`
	OP       float64 `json:"op"`
	TSUnixMs int64   `json:"ts"`
}

// sensorBuffer accumulates samples for one sensor until a full window is
// ready. ts is tracked per-sample (not just a single start field) because
// with sliding windows the buffer never fully empties between emissions --
// the oldest remaining sample's timestamp is what each new window starts at.
type sensorBuffer struct {
	mu sync.Mutex
	pv []float64
	op []float64
	ts []int64
}

type ingestor struct {
	mu       sync.Mutex
	buffers  map[string]*sensorBuffer
	detector pb.DetectionClient
}

func newIngestor(client pb.DetectionClient) *ingestor {
	return &ingestor{buffers: make(map[string]*sensorBuffer), detector: client}
}

func (in *ingestor) handleSample(s Sample) {
	in.mu.Lock()
	buf, ok := in.buffers[s.SensorID]
	if !ok {
		buf = &sensorBuffer{}
		in.buffers[s.SensorID] = buf
	}
	in.mu.Unlock()

	buf.mu.Lock()
	buf.pv = append(buf.pv, s.PV)
	buf.op = append(buf.op, s.OP)
	buf.ts = append(buf.ts, s.TSUnixMs)

	var pvWindow, opWindow []float64
	var windowStart int64
	full := len(buf.pv) >= WindowSize
	if full {
		pvWindow = append([]float64(nil), buf.pv[:WindowSize]...)
		opWindow = append([]float64(nil), buf.op[:WindowSize]...)
		windowStart = buf.ts[0]
		// slide forward by WindowStride; stride == WindowSize (the
		// default) reduces to the original non-overlapping behavior
		buf.pv = append([]float64(nil), buf.pv[WindowStride:]...)
		buf.op = append([]float64(nil), buf.op[WindowStride:]...)
		buf.ts = append([]int64(nil), buf.ts[WindowStride:]...)
	}
	buf.mu.Unlock()

	if full {
		in.sendWindow(s.SensorID, pvWindow, opWindow, windowStart)
	}
}

func (in *ingestor) sendWindow(sensorID string, pv, op []float64, windowStart int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := in.detector.DetectWindow(ctx, &pb.WindowRequest{
		SensorId:          sensorID,
		Pv:                pv,
		Op:                op,
		WindowStartUnixMs: windowStart,
	})
	if err != nil {
		log.Printf("[%s] detection request failed: %v", sensorID, err)
		return
	}
	log.Printf(
		"[%s] label=%s ellipse_index=%.3f kano=%v has_activity=%v",
		sensorID, resp.Label, resp.EllipseIndex, resp.KanoVerdict, resp.HasActivity,
	)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	brokerURL := getenv("MQTT_BROKER_URL", "tcp://localhost:1883")
	topic := getenv("MQTT_TOPIC", "valve/data")
	detectionAddr := getenv("DETECTION_SERVICE_ADDR", "localhost:50051")

	if strideStr := os.Getenv("WINDOW_STRIDE"); strideStr != "" {
		stride, err := strconv.Atoi(strideStr)
		if err != nil || stride <= 0 || stride > WindowSize {
			log.Fatalf("WINDOW_STRIDE must be an integer in (0, %d], got %q", WindowSize, strideStr)
		}
		WindowStride = stride
	}
	log.Printf("window_size=%d window_stride=%d", WindowSize, WindowStride)

	conn, err := grpc.NewClient(detectionAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to detection service at %s: %v", detectionAddr, err)
	}
	defer conn.Close()
	client := pb.NewDetectionClient(conn)
	in := newIngestor(client)

	opts := mqtt.NewClientOptions().AddBroker(brokerURL).SetClientID("ingestion-service")
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Printf("connected to MQTT broker %s, subscribing to %s", brokerURL, topic)
		token := c.Subscribe(topic, 1, func(_ mqtt.Client, msg mqtt.Message) {
			var s Sample
			if err := json.Unmarshal(msg.Payload(), &s); err != nil {
				log.Printf("failed to parse message: %v", err)
				return
			}
			in.handleSample(s)
		})
		token.Wait()
	})

	client_ := mqtt.NewClient(opts)
	if token := client_.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("failed to connect to MQTT broker: %v", token.Error())
	}

	select {} // block forever
}
