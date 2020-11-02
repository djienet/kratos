package blademaster

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var defaultMetricPath = "/metrics"

//RequestCounterURLLabelMappingFn 请求路径URL规则匹配函数，用于支持自定义自己的请求路径匹配方式
//因为类似商品详情页、分类页等页面为了seo做了页面静态化，但实际上都是同一个请求，因此需要自定义路径
//例如，category/100.html 匹配成category/:id.html
type RequestCounterURLLabelMappingFn func(c *Context) string

// Prometheus contains the metrics gathered by the instance and its path
type Prometheus struct {
	router        *Engine
	listenAddress string

	MetricsPath string

	ReqCntURLLabelMappingFn RequestCounterURLLabelMappingFn
}

// NewPrometheus generates a new set of metrics with a certain subsystem name
func NewPrometheus() *Prometheus {

	p := &Prometheus{
		MetricsPath: defaultMetricPath,
		ReqCntURLLabelMappingFn: func(c *Context) string {
			return c.Request.URL.Path // 默认返回源请求路径
		},
	}

	return p
}

func getMetrics() HandlerFunc {
	return func(c *Context) {
		h := promhttp.Handler()
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// calcaute request counter and duration
func (p *Prometheus) handlerFunc() HandlerFunc {
	return func(c *Context) {
		if c.Request.URL.String() == p.MetricsPath {
			c.Next()
			return
		}

		start := time.Now()
		reqSz := make(chan int)
		go computeApproximateRequestSize(c.Request, reqSz)
		reqSize := float64(<-reqSz)

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		elapsed := float64(time.Since(start)) / float64(time.Second)
		resSz := float64(c.Writer.Size())

		requestPath := p.ReqCntURLLabelMappingFn(c)

		_metricsRequestionDurationSeconds.Observe(elapsed, requestPath)
		_metricsRequestionTotal.Inc(status, c.Request.Method, requestPath, c.Request.Host)
		_metrcisRequestionSize.Observe(reqSize, requestPath)
		_metricsResponseSize.Observe(resSz, requestPath)
	}
}

// From https://github.com/DanielHeckrath/gin-prometheus/blob/master/gin_prometheus.go
func computeApproximateRequestSize(r *http.Request, out chan int) {
	s := 0
	if r.URL != nil {
		s = len(r.URL.String())
	}

	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	// N.B. r.Form and r.MultipartForm are assumed to be included in r.URL.

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}

	out <- s
}
