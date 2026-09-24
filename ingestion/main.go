// Ingestion service (PRD FR-2): subscribes to MQTT, buffers each sensor's
// PV/OP into fixed-size windows, forwards full windows to the detection
// service over gRPC.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "valve-stiction-pipeline/ingestion/detectionpb"
)

// WindowSize matches what valve-stiction-ml validated the classic detector
// against (see its ML_PLAN.md §13) -- not an arbitrary choice.
const WindowSize = 100

type Sample struct {
	SensorID string  `json:"sensor_id"`
	PV       float64 `json:"pv"`
	OP       float64 `json:"op"`
	TSUnixMs int64   `json:"ts"`
}

// sensorBuffer accumulates samples for one sensor until a full window is ready.
type sensorBuffer struct {
	mu    sync.Mutex
	pv    []float64
	op    []float64
	start int64
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
		buf = &sensorBuffer{start: s.TSUnixMs}
		in.buffers[s.SensorID] = buf
	}
	in.mu.Unlock()

	buf.mu.Lock()
	if len(buf.pv) == 0 {
		buf.start = s.TSUnixMs
	}
	buf.pv = append(buf.pv, s.PV)
	buf.op = append(buf.op, s.OP)

	var pvWindow, opWindow []float64
	var windowStart int64
	full := len(buf.pv) >= WindowSize
	if full {
		pvWindow = append([]float64(nil), buf.pv[:WindowSize]...)
		opWindow = append([]float64(nil), buf.op[:WindowSize]...)
		windowStart = buf.start
		// non-overlapping windows, matching training exactly (see docs/V1_PLAN.md)
		buf.pv = buf.pv[:0]
		buf.op = buf.op[:0]
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
