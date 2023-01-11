package collector

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricType uint8

const (
	// Gauge is a metric that represents a single numerical value that can arbitrarily go up and down.
	// during metric Update the collected value is set, thus previous value is overwritten.
	Gauge MetricType = iota
	GaugeVec
	// Counter is a cumulative metric that represents a single numerical value that only ever goes up.
	// during metric Update the collected value is added to its current value.
	Counter
	CounterVec
)

// Metric is a single metric that will be registered to prometheus registry and collected with WithCollectFunc callback.
// Metric can be collected periodically based on metric collection rate of prometheus or WithUpdateOnEvent callback
// can be provided, so the Metric will keep its internal representation of metrics value,
// and WithCollectFunc will use it instead requesting data directly form other components.
type Metric struct {
	Name          string
	Type          MetricType
	Namespace     string
	help          string
	labels        []string
	collectFunc   func() map[string]float64
	initValueFunc func() map[string]float64
	initFunc      func()

	PromMetric                   prometheus.Collector
	resetEnabled                 bool // if enabled metric will be reset before each collectFunction call
	labelValuesCollectionEnabled bool // if enabled metric will use UpdateWithLabels instead of Update

	once sync.Once
}

// NewMetric creates a new metric with given name and options.
func NewMetric(name string, opts ...options.Option[Metric]) *Metric {
	m := options.Apply(&Metric{
		Name:        name,
		collectFunc: func() map[string]float64 { return nil },
	}, opts)

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

// Collect calls the collectFunc and updates the metric value, for Gauge/GaugeVec values are set,
// for Counter/CounterVec values are added.
func (m *Metric) Collect() {
	valMap := m.collectFunc()
	if valMap != nil {
		m.Update(valMap)
	}
}

// Update updates the metric value, for Gauge/GaugeVec values are set, for Counter/CounterVec values are added.
// For CounterVec/GaugeVec metrics updates will be done only if each provided metric label was previously defined with WithLabels option.
// To set metrics labels string values, enable use WithLabelValuesCollection option.
func (m *Metric) Update(values map[string]float64) {
	if m.labelValuesCollectionEnabled {
		value := float64(0)
		labelValues := make([]string, 0)
		for label, val := range values {
			labelValues = append(labelValues, label)
			value = val
		}
		if len(labelValues) != len(m.labels) {
			fmt.Println("nothing updated, label values and labels length mismatch when updating metric", m.Name)
			return
		}
		m.updateWithLabels(labelValues, value)
	} else {
		for _, label := range m.labels {
			if _, ok := values[label]; ok {
				// we allow to provide only values for previously defined labels, to keep data cardinality under control
				// to set string values by adding new labels use WithLabelValuesCollection option.
				fmt.Println("Metric", m.Name, "tries to set value for a label that is not defined in metric options, metric not updated")
				return
			}
		}
		m.updateWithValues(values)
	}
}

// updateWithValues updates the metric value, for Gauge/GaugeVec values are set, for Counter/CounterVec values are added.
// To set metrics labels string values, enable use WithLabelValuesCollection option.
func (m *Metric) updateWithValues(values map[string]float64) {
	fmt.Println("updateWithValues metric", m.Name, "with values", values)
	switch m.Type {
	case Gauge:
		for _, val := range values {
			m.PromMetric.(prometheus.Gauge).Set(val)
			break
		}
	case GaugeVec:
		for label, val := range values {
			m.PromMetric.(*prometheus.GaugeVec).WithLabelValues(label).Set(val)
		}
	case Counter:
		for _, val := range values {
			m.PromMetric.(prometheus.Counter).Add(val)
			break
		}
	case CounterVec:
		for label, val := range values {
			m.PromMetric.(*prometheus.CounterVec).WithLabelValues(label).Add(val)
		}
	}
}

// UpdateWithLabels allows to add new label values (string) and set/add value for them for correspondingly GaugeVec/CounterVec.
// Provided label values should be ordered correspondingly to m.labels provided with WithLabels option.
func (m *Metric) updateWithLabels(labelValues []string, val float64) {
	fmt.Println("updateWithLabels metric", m.Name, "with labels", labelValues)
	switch m.Type {
	case Gauge:
	case GaugeVec:
		m.PromMetric.(*prometheus.GaugeVec).WithLabelValues(labelValues...).Set(val)
	case Counter:
	case CounterVec:
		m.PromMetric.(*prometheus.CounterVec).WithLabelValues(labelValues...).Add(val)
	}
}

func (m *Metric) Increment(labels ...string) {
	fmt.Println("Incrementing metric", m.Name)
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

func WithLabelValuesCollection() options.Option[Metric] {
	return func(m *Metric) {
		m.labelValuesCollectionEnabled = true
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
