package trace

import (
	"io"
	"time"

	"github.com/djienet/kratos/pkg/conf/env"

	opentracing "github.com/opentracing/opentracing-go"
	jaeger "github.com/uber/jaeger-client-go"
	jaegerconfig "github.com/uber/jaeger-client-go/config"

	protogen "github.com/djienet/kratos/pkg/net/trace/proto"
)

type report struct {
	tracer opentracing.Tracer
	close  io.Closer
}

func newReport(agentHostPort string) *report {
	cfg := jaegerconfig.Configuration{
		Disabled: false,
		Sampler: &jaegerconfig.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &jaegerconfig.ReporterConfig{
			LogSpans:            false,
			BufferFlushInterval: 1 * time.Second,
			LocalAgentHostPort:  agentHostPort,
		},
	}

	serviceName := env.AppID
	if serviceName == "" {
		serviceName = "-"
	}

	tracer, close, _ := cfg.New(serviceName)

	return &report{
		tracer: tracer,
		close:  close,
	}
}

//Send 把span发送到jaeger中
func (rpt *report) WriteSpan(raw *Span) error {
	spanCtx, err := jaeger.ContextFromString(raw.Context().String())

	if err != nil {
		//log.Error("cannot initialize spanContext: %v", err)
		// panic(err)
		return err
	}

	span := rpt.tracer.StartSpan(raw.OperationName(), jaeger.SelfRef(spanCtx), opentracing.StartTime(raw.StartTime()))

	setTag(span, raw.Tags())
	setLog(span, raw.Logs())

	span.Finish()
	return nil
}

// 把dapper的tag转化为jaeger的tag
func setTag(span opentracing.Span, tags []Tag) {
	duplicateTags := make(map[string]interface{})

	for _, tag := range tags {
		// 有一些tag标签会重复，因此这里过滤掉
		if duplicateTags[tag.Key] != nil {
			continue
		}
		duplicateTags[tag.Key] = tag.Value

		span.SetTag(tag.Key, tag.Value)
	}
}

// 把dapper的log转化为jaeger的log
func setLog(span opentracing.Span, logs []*protogen.Log) {
	for _, _log := range logs {
		for _, field := range _log.Fields {
			logData := opentracing.LogData{
				Timestamp: time.Unix(0, _log.Timestamp),
				Event:     field.Key,
				Payload:   string(field.Value),
			}
			span.Log(logData)
		}
	}
}

//Close 关闭释放span
func (rpt *report) Close() error {
	return rpt.close.Close()
}
