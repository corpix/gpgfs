package collector

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"git.backbone/corpix/gpgfs/pkg/meta"
)

var (
	NewCounter      = prometheus.NewCounter
	NewCounterVec   = prometheus.NewCounterVec
	NewGauge        = prometheus.NewGauge
	NewGaugeVec     = prometheus.NewGaugeVec
	NewHistogram    = prometheus.NewHistogram
	NewHistogramVec = prometheus.NewHistogramVec
)

type (
	Counter     = prometheus.Counter
	CounterVec  = prometheus.CounterVec
	CounterOpts = prometheus.CounterOpts

	Gauge     = prometheus.Gauge
	GaugeVec  = prometheus.GaugeVec
	GaugeOpts = prometheus.GaugeOpts

	Histogram     = prometheus.Histogram
	HistogramVec  = prometheus.HistogramVec
	HistogramOpts = prometheus.HistogramOpts

	Labels = prometheus.Labels
)

func Name(subsystem string, name string, rest ...string) string {
	return strings.Join(
		append(
			[]string{meta.TelemetryNamespace, subsystem, name},
			rest...,
		),
		"_",
	)
}
