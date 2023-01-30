package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Collector is responsible for creation and collection of metrics for the prometheus.
type Collector struct {
	Registry    *prometheus.Registry
	collections map[string]*Collection
}

// New creates an instance of Manager and creates a new prometheus registry for the protocol metrics collection.
func New() *Collector {
	return &Collector{
		Registry:    prometheus.NewRegistry(),
		collections: make(map[string]*Collection),
	}
}

func (c *Collector) RegisterCollection(coll *Collection) {
	c.collections[coll.CollectionName] = coll
	for _, m := range coll.metrics {
		c.Registry.MustRegister(m.PromMetric)
		if m.initValueFunc != nil {
			m.Update(m.initValueFunc())
		}
		if m.initFunc != nil {
			m.initFunc()
		}
	}
}

func (c *Collector) Collect() {
	for _, collection := range c.collections {
		for _, metric := range collection.metrics {
			metric.Collect()
		}
	}
}

func (c *Collector) getMetric(subsystem, metricName string) *Metric {
	col := c.getCollection(subsystem)
	if col != nil {
		return col.GetMetric(metricName)
	}
	return nil
}

func (c *Collector) getCollection(subsystem string) *Collection {
	if collection, exists := c.collections[subsystem]; exists {
		return collection
	}
	return nil
}

// Update updates the value of the existing metric defined by the subsystem and metricName.
func (c *Collector) Update(subsystem, metricName string, labelValues map[string]float64) {
	m := c.getMetric(subsystem, metricName)
	if m != nil {
		m.Update(labelValues)
	}
}

func (c *Collector) Increment(subsystem, metricName string, labels ...string) {
	m := c.getMetric(subsystem, metricName)
	if m != nil {
		m.Increment(labels...)
	}
}

func (c *Collector) ResetMetricLabels(subsystem, metricName string, labelValues map[string]string) {
	m := c.getMetric(subsystem, metricName)
	if m != nil {
		m.ResetLabels(labelValues)
	}
}

func (c *Collector) ResetMetric(namespace string, node string) {
	m := c.getMetric(namespace, node)
	if m != nil {
		m.Reset()
	}
}
