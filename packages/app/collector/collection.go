package collector

import (
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/options"
)

const singleValLabel = "value"

type Collection struct {
	CollectionName string
	metrics        map[string]*Metric
}

func NewCollection(name string, opts ...options.Option[Collection]) *Collection {
	return options.Apply(&Collection{
		CollectionName: name,
		metrics:        make(map[string]*Metric),
	}, opts, func(c *Collection) {
		for _, m := range c.metrics {
			m.Namespace = c.CollectionName
			m.initPromMetric()
		}
	})
}

func (c *Collection) GetMetric(metricName string) *Metric {
	if metric, exists := c.metrics[metricName]; exists {
		return metric
	}
	return nil
}

func (c *Collection) addMetric(metric *Metric) {
	if metric != nil {
		c.metrics[metric.Name] = metric
	}
}

// region Options ///////////////////////////////////////////////////////////////////////////////////////////////////////

func WithMetric(metric *Metric) options.Option[Collection] {
	return func(c *Collection) {
		c.addMetric(metric)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func SingleValue[T constraints.Numeric](val T) map[string]float64 {
	return map[string]float64{
		singleValLabel: float64(val),
	}
}

func MultiValue[T constraints.Numeric](labels []string, values ...T) map[string]float64 {
	if len(labels) != len(values) {
		return nil
	}
	m := make(map[string]float64)
	for i, label := range labels {
		m[label] = float64(values[i])
	}
	return m
}
