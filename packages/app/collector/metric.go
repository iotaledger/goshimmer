package collector

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricType uint8

const (
	Gauge MetricType = iota
	GaugeVec
	Counter
)

// Metric is a single metric that will be registered to prometheus registry and collected with WithCollectFunc callback.
// Metric can be collected periodically based on metric collection rate of prometheus or WithUpdateOnEvent callback
// can be provided, so the Metric will keep its internal representation of metrics value,
// and WithCollectFunct will use it instead requesting data directly form other components.
type Metric struct {
	Name        string
	Type        MetricType
	help        string
	subsystem   string
	collectFunc func() float64
	updateFunc  func()
	initFunc    func()

	internalStateNeeded bool

	promMetric prometheus.Metric
}

// NewMetric creates a new metric with given name and options.
func NewMetric(name string, opts ...options.Option[Metric]) *Metric {
	m := options.Apply(&Metric{
		Name: name,
	}, opts)

	switch m.Type {
	case Gauge:
		m.promMetric = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:      name,
			Subsystem: m.subsystem,
		})
	}
	return m
}

func (m *Metric) Collect() {
	val := m.collectFunc()
	switch m.Type {
	case Gauge:
		m.promMetric.(prometheus.Gauge).Set(val)
	}

}

// region Options ///////////////////////////////////////////////////////////////////////////////////////////////////////

func WithType(t MetricType) options.Option[Metric] {
	return func(m *Metric) {
		m.Type = t
	}
}

func WithHelp(help string) options.Option[Metric] {
	return func(m *Metric) {
		m.help = help
	}
}

func WithCollectFunc(collectFunc func() float64) options.Option[Metric] {
	return func(m *Metric) {
		m.collectFunc = collectFunc
	}
}

func WithUpdateOnEvent(updateFunc func()) options.Option[Metric] {
	return func(m *Metric) {
		m.internalStateNeeded = true
		m.updateFunc = updateFunc
	}
}

func WithInitFunc(initFunc func()) options.Option[Metric] {
	return func(m *Metric) {
		m.internalStateNeeded = true
		m.initFunc = initFunc
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
