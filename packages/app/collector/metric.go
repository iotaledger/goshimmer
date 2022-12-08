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
	CounterVec
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
	labels      []string
	collectFunc func() map[string]float64
	updateFunc  func()
	initFunc    func()

	promMetric prometheus.Collector
}

// NewMetric creates a new metric with given name and options.
func NewMetric(name string, opts ...options.Option[Metric]) *Metric {
	m := options.Apply(&Metric{
		Name: name,
	}, opts, emptyCollector)

	switch m.Type {
	case Gauge:
		m.promMetric = prometheus.NewGauge(prometheus.GaugeOpts{
			Name:      name,
			Subsystem: m.subsystem,
			Help:      m.help,
		})
	case GaugeVec:
		m.promMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:      name,
			Subsystem: m.subsystem,
			Help:      m.help,
		}, m.labels)
	case Counter:
		m.promMetric = prometheus.NewCounter(prometheus.CounterOpts{
			Name:      name,
			Subsystem: m.subsystem,
			Help:      m.help,
		})
	case CounterVec:
		m.promMetric = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:      name,
			Subsystem: m.subsystem,
			Help:      m.help,
		}, m.labels)
	}
	return m
}

func (m *Metric) Collect() {
	valMap := m.collectFunc()
	if valMap != nil {
		m.Update(valMap)
	}
}

func (m *Metric) Update(labelValues map[string]float64) {
	switch m.Type {
	case Gauge:
		for _, val := range labelValues {
			m.promMetric.(prometheus.Gauge).Set(val)
			break
		}
	case GaugeVec:
		for label, val := range labelValues {
			m.promMetric.(*prometheus.GaugeVec).WithLabelValues(label).Set(val)
		}
	case Counter:
		for _, val := range labelValues {
			m.promMetric.(prometheus.Counter).Add(val)
			break
		}
	case CounterVec:
		for label, val := range labelValues {
			m.promMetric.(*prometheus.CounterVec).WithLabelValues(label).Add(val)
		}
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

func WithLabels(labels ...string) options.Option[Metric] {
	return func(m *Metric) {
		m.labels = labels
	}
}

func WithCollectFunc(collectFunc func() map[string]float64) options.Option[Metric] {
	return func(m *Metric) {
		m.collectFunc = collectFunc
	}
}

func WithUpdateOnEvent(updateFunc func()) options.Option[Metric] {
	return func(m *Metric) {
		m.updateFunc = updateFunc
	}
}

func WithInitFunc(initFunc func()) options.Option[Metric] {
	return func(m *Metric) {
		m.initFunc = initFunc
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func emptyCollector(m *Metric) {
	m.collectFunc = func() map[string]float64 { return nil }
}
