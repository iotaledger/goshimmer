package collector

import (
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/runtime/options"
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

// SingleValue is a helper function to create a map of labels and values for a single value, where the label will be "value".
func SingleValue[T constraints.Numeric](val T) map[string]float64 {
	return map[string]float64{
		singleValLabel: float64(val),
	}
}

// MultiLabelsValues is a helper function to create a map of labels and values for given labels and values.
func MultiLabelsValues[T constraints.Numeric](labels []string, values ...T) map[string]float64 {
	if len(labels) != len(values) {
		return nil
	}
	m := make(map[string]float64)
	for i, label := range labels {
		m[label] = float64(values[i])
	}
	return m
}

// MultiLabels is a helper function to create a map of labels with preserved order when WithLabelValuesCollection is enabled for metric.
func MultiLabels(labels ...string) map[string]float64 {
	m := make(map[string]float64)
	for order, label := range labels {
		m[label] = float64(order)
	}
	return m
}
