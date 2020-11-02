package metric

import (
	"github.com/prometheus/client_golang/prometheus"
)

// SummaryVecOpts is histogram vector opts.
type SummaryVecOpts struct {
	Namespace string
	Subsystem string
	Name      string
	Help      string
	Labels    []string
}

// SummaryVec gauge vec.
type SummaryVec interface {
	// Observe adds a single observation to the summary.
	Observe(v float64, labels ...string)
}

// Summary prom histogram collection.
type promSummaryVec struct {
	summary *prometheus.SummaryVec
}

// NewSummaryVec new a histogram vec.
func NewSummaryVec(cfg *SummaryVecOpts) SummaryVec {
	if cfg == nil {
		return nil
	}
	vec := prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      cfg.Name,
			Help:      cfg.Help,
		}, cfg.Labels)
	prometheus.MustRegister(vec)
	return &promSummaryVec{
		summary: vec,
	}
}

// Timing adds a single observation to the histogram.
func (summary *promSummaryVec) Observe(v float64, labels ...string) {
	summary.summary.WithLabelValues(labels...).Observe(v)
}
