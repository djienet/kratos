package blademaster

import "github.com/djienet/kratos/pkg/stat/metric"

const (
	clientNamespace = "http_client"
	serverNamespace = "http_server"
)

var (
	_metricServerReqDur = metric.NewHistogramVec(&metric.HistogramVecOpts{
		Namespace: serverNamespace,
		Subsystem: "requests",
		Name:      "duration_ms",
		Help:      "http server requests duration(ms).",
		Labels:    []string{"path", "caller", "method"},
		Buckets:   []float64{5, 10, 25, 50, 100, 250, 500, 1000},
	})
	_metricServerReqCodeTotal = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: serverNamespace,
		Subsystem: "requests",
		Name:      "code_total",
		Help:      "http server requests error count.",
		Labels:    []string{"path", "caller", "method", "code"},
	})
	_metricServerBBR = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: serverNamespace,
		Subsystem: "",
		Name:      "bbr_total",
		Help:      "http server bbr total.",
		Labels:    []string{"url", "method"},
	})
	_metricClientReqDur = metric.NewHistogramVec(&metric.HistogramVecOpts{
		Namespace: clientNamespace,
		Subsystem: "requests",
		Name:      "duration_ms",
		Help:      "http client requests duration(ms).",
		Labels:    []string{"path", "method"},
		Buckets:   []float64{5, 10, 25, 50, 100, 250, 500, 1000},
	})
	_metricClientReqCodeTotal = metric.NewCounterVec(&metric.CounterVecOpts{
		Namespace: clientNamespace,
		Subsystem: "requests",
		Name:      "code_total",
		Help:      "http client requests code count.",
		Labels:    []string{"path", "method", "code"},
	})

	_metricsRequestionTotal = metric.NewCounterVec(&metric.CounterVecOpts{
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "How many HTTP requests processed, partitioned by status code and HTTP request path.",
		Labels:    []string{"code", "method", "request_path", "host"},
	})

	_metricsRequestionDurationSeconds = metric.NewSummaryVec(&metric.SummaryVecOpts{
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "The HTTP request latencies in seconds.",
		Labels:    []string{"request_path"},
	})

	_metrcisRequestionSize = metric.NewSummaryVec(&metric.SummaryVecOpts{
		Subsystem: "http",
		Name:      "request_size_bytes",
		Help:      "The HTTP request sizes in bytes.",
		Labels:    []string{"request_path"},
	})

	_metricsResponseSize = metric.NewSummaryVec(&metric.SummaryVecOpts{
		Subsystem: "http",
		Name:      "response_size_bytes",
		Help:      "The HTTP response sizes in bytes.",
		Labels:    []string{"request_path"},
	})
)
