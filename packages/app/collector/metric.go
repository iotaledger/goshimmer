package collector

import (
	"sync"

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
	Name          string
	Type          MetricType
	Namespace     string
	help          string
	labels        []string
	collectFunc   func() map[string]float64
	updateFunc    func()
	initValueFunc func() map[string]float64
	initFunc      func()

	PromMetric   prometheus.Collector
	resetEnabled bool // if enabled metric will be reset before each collectFunction call

	once sync.Once
}

// NewMetric creates a new metric with given name and options.
func NewMetric(name string, opts ...options.Option[Metric]) *Metric {
	m := options.Apply(&Metric{
		Name: name,
	}, opts, emptyCollector)

	return m
}

func (m *Metric) initPromMetric() {
	m.once.Do(func() {
		switch m.Type {
		case Gauge:
			m.PromMetric = prometheus.NewGauge(prometheus.GaugeOpts{
				Name:      m.Name,
				Namespace: m.Namespace,
				Help:      m.help,
			})
		case GaugeVec:
			m.PromMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name:      m.Name,
				Namespace: m.Namespace,
				Help:      m.help,
			}, m.labels)
		case Counter:
			m.PromMetric = prometheus.NewCounter(prometheus.CounterOpts{
				Name:      m.Name,
				Namespace: m.Namespace,
				Help:      m.help,
			})
		case CounterVec:
			m.PromMetric = prometheus.NewCounterVec(prometheus.CounterOpts{
				Name:      m.Name,
				Namespace: m.Namespace,
				Help:      m.help,
			}, m.labels)
		}
	})

}

func (m *Metric) Collect() {
	valMap := m.collectFunc()
	if valMap != nil {
		m.Update(valMap)
	}
}

// todo separate update from add
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

func (m *Metric) Increment(labels ...string) {
	if len(labels) == 0 {
		switch m.Type {
		case Gauge:
			m.PromMetric.(prometheus.Gauge).Inc()
		case GaugeVec:
			m.PromMetric.(*prometheus.GaugeVec).WithLabelValues().Inc()
		case Counter:
			m.PromMetric.(prometheus.Counter).Inc()
		case CounterVec:
			m.PromMetric.(*prometheus.CounterVec).WithLabelValues().Inc()
		}
	} else {
		switch m.Type {
		case GaugeVec:
			m.PromMetric.(*prometheus.GaugeVec).WithLabelValues(labels...).Inc()
		case CounterVec:
			m.PromMetric.(*prometheus.CounterVec).WithLabelValues(labels...).Inc()
		}
	}
}

func (m *Metric) Reset() {
	switch m.Type {
	case Gauge:
		m.PromMetric.(prometheus.Gauge).Set(0)
	case GaugeVec:
		m.PromMetric.(*prometheus.GaugeVec).Reset()
	case Counter:
		m.PromMetric = prometheus.NewCounter(prometheus.CounterOpts{
			Name:      m.Name,
			Namespace: m.Namespace,
			Help:      m.help,
		})
	case CounterVec:
		m.PromMetric.(*prometheus.CounterVec).Reset()
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

// WithResetBeforeCollecting  if enabled there will be a reset call on metric before each collectFunction call.
// TODO handle this logic rather from metric level, not the entire collection
func WithResetBeforeCollecting(resetEnabled bool) options.Option[Metric] {
	return func(m *Metric) {
		m.resetEnabled = resetEnabled
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

func WithInitValue(initValueFunc func() map[string]float64) options.Option[Metric] {
	return func(m *Metric) {
		m.initValueFunc = initValueFunc
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func emptyCollector(m *Metric) {
	m.collectFunc = func() map[string]float64 { return nil }
}
