package lightstep

import (
	"context"
	"encoding/binary"
	"sync"

	"github.com/opentracing/opentracing-go/log"

	"go.opentelemetry.io/api/core"
	"go.opentelemetry.io/sdk/export"
	"go.opentelemetry.io/sdk/trace"

	"github.com/opentracing/opentracing-go"

	"github.com/lightstep/lightstep-tracer-go"
	lsclient "github.com/lightstep/lightstep-tracer-go"
)

type Config struct {
	AccessToken string
	Host        string
	Port        int
	ServiceName string
}

type Exporter struct {
	once   sync.Once
	tracer lsclient.Tracer
}

func marshalConfigToOptions(c Config) lsclient.Options {
	opts := lsclient.Options{}
	opts.AccessToken = c.AccessToken
	opts.Collector.Host = c.Host
	opts.Collector.Port = c.Port
	opts.Collector.Plaintext = false
	opts.Initialize()
	return opts
}

func NewExporter(config Config) (*Exporter, error) {
	tracerOptions := marshalConfigToOptions(config)
	tracer := lsclient.NewTracer(tracerOptions)

	return &Exporter{
		tracer: tracer,
	}, nil
}

func (e *Exporter) RegisterSimpleSpanProcessor() {
	e.once.Do(func() {
		ssp := trace.NewSimpleSpanProcessor(e)
		trace.RegisterSpanProcessor(ssp)
	})
}

func (e *Exporter) ExportSpan(ctx context.Context, data *export.SpanData) {
	e.tracer.StartSpan(
		data.Name,
		lightstep.SetTraceID(convertTraceID(data.SpanContext.TraceID)),
		lightstep.SetSpanID(convertSpanID(data.SpanContext.SpanID)),
		lightstep.SetParentSpanID(convertSpanID(data.ParentSpanID)),
		opentracing.StartTime(data.StartTime),
		opentracing.Tags(toTags(data.Attributes)),
	).FinishWithOptions(
		opentracing.FinishOptions{
			FinishTime: data.EndTime,
			LogRecords: toLogRecords(data.MessageEvents),
		},
	)
}

func (e *Exporter) Close() {
	e.tracer.Close(context.Background())
}

func convertTraceID(id core.TraceID) uint64 {
	first := binary.LittleEndian.Uint64(id[:8])
	second := binary.LittleEndian.Uint64(id[8:])
	return first ^ second
}

func convertSpanID(id core.SpanID) uint64 {
	return binary.LittleEndian.Uint64(id[:])
}

func toLogRecords(input []export.Event) []opentracing.LogRecord {
	output := make([]opentracing.LogRecord, 0, len(input))
	for _, l := range input {
		output = append(output, toLogRecord(l))
	}
	return output
}

func toTags(input []core.KeyValue) map[string]interface{} {
	output := make(map[string]interface{})
	for _, value := range input {
		output[string(value.Key)] = value.Value.AsInterface()
	}
	return output
}

func toLogRecord(ev export.Event) opentracing.LogRecord {
	return opentracing.LogRecord{
		Timestamp: ev.Time,
		Fields:    toFields(ev.Attributes),
	}
}

func toFields(input []core.KeyValue) []log.Field {
	output := make([]log.Field, 0, len(input))
	for _, value := range input {
		output = append(output, log.Object(string(value.Key), value.Value.AsInterface()))
	}
	return output
}
