package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type OTLPExporter struct {
	endpoint string //http://localhost:4318/v1/traces
	client   *http.Client
	ch       chan *Span //recieves span
	batch    []*Span    //accumulation before flushing
	wg       sync.WaitGroup
	mu       sync.Mutex //protects batch from concurrent access
	quit     chan struct{}
}

func NewExporter(endpoint string) *OTLPExporter {
	return &OTLPExporter{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 5 * time.Second},
		ch:       make(chan *Span, 512),
		batch:    make([]*Span, 0, 512),
		quit:     make(chan struct{}),
	}
}

func (e *OTLPExporter) Start() {
	e.wg.Add(1)
	go e.process()
}

//middleware calls this after every req
func (e *OTLPExporter) Export(s *Span) {
	select {
	case e.ch <- s:
		//queued
	default:
		fmt.Println("[EXPORTER] buffer full, dropping span:", s.Name)
	}
}

func (e *OTLPExporter) process() {
	defer e.wg.Done()

	t := time.NewTicker(5 * time.Second) //returns time on a channel
	defer t.Stop()

	for {
		select {
		case span := <-e.ch:
			e.mu.Lock()
			e.batch = append(e.batch, span)
			full := len(e.batch) >= 512
			e.mu.Unlock()

			if full {
				//flush
				e.flush()
			}

		case <-t.C:
			//flush
			e.flush()

		case <-e.quit:
			for {
				select {
				case span := <-e.ch:
					e.mu.Lock()
					e.batch = append(e.batch, span)
					e.mu.Unlock()

				default:
					return
				}
			}
		}
	}
}

func (e *OTLPExporter) flush() error {
	e.mu.Lock()
	if len(e.batch) == 0 {
		e.mu.Unlock()
		return nil
	}
	spans := e.batch
	e.batch = make([]*Span, 0, 512)
	e.mu.Unlock()

	//build otlp payload
	payload := buildOTLPPayload(spans)

	//marshalling payload
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	//post request
	res, err := e.client.Post(e.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("export error: %w", err)
	}
	defer res.Body.Close()

	fmt.Printf("[EXPORTER] flushed %d spans → %d\n", len(spans), res.StatusCode)

	return nil
}

var ServiceName = "observe"

//converts our spans into OTLP JSON structure
//TODO: look into it
func buildOTLPPayload(spans []*Span) map[string]any {
	otlpSpans := make([]map[string]any, 0, len(spans))

	for _, s := range spans {
		status := map[string]any{"code": 0}
		switch s.Status {
		case StatusError:
			status["code"] = 2
		case StatusOK:
			status["code"] = 1
		}

		attrs := make([]map[string]any, 0)
		for k, v := range s.Attributes {
			attrs = append(attrs, map[string]any{
				"key":   k,
				"value": map[string]any{"stringValue": v},
			})
		}

		events := make([]map[string]any, 0)
		for _, ev := range s.Events {
			evAttrs := make([]map[string]any, 0)
			for k, v := range ev.Attrs {
				evAttrs = append(evAttrs, map[string]any{
					"key":   k,
					"value": map[string]any{"stringValue": v},
				})
			}
			events = append(events, map[string]any{
				"name":         ev.Name,
				"timeUnixNano": fmt.Sprintf("%d", ev.TimeStamp.UnixNano()),
				"attributes":   evAttrs,
			})
		}

		otlpSpans = append(otlpSpans, map[string]any{
			"traceId":           s.TraceID,
			"spanId":            s.SpanID,
			"parentSpanId":      s.ParentID,
			"name":              s.Name,
			"kind":              2,
			"startTimeUnixNano": fmt.Sprintf("%d", s.StartTime.UnixNano()),
			"endTimeUnixNano":   fmt.Sprintf("%d", s.EndTime.UnixNano()),
			"attributes":        attrs,
			"events":            events,
			"status":            status,
		})
	}

	return map[string]any{
		"resourceSpans": []map[string]any{
			{
				"resource": map[string]any{
					"attributes": []map[string]any{
						{
							"key":   "service.name",
							"value": map[string]any{"stringValue": ServiceName},
						},
					},
				},
				"scopeSpans": []map[string]any{
					{
						"spans": otlpSpans,
					},
				},
			},
		},
	}
}

func (e *OTLPExporter) Shutdown(ctx context.Context) error {
	close(e.quit)
	e.wg.Wait()
	return e.flush()
}
