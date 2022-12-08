package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Collector is responsible for creation and collection of metrics for the prometheus.
type Collector struct {
	Registry    *prometheus.Registry
	collections map[string]*Collection
}

// New creates an instance of Manger, creates new prometheus registry for the protocol metrics collection.
func New() *Collector {
	return &Collector{
		Registry:    prometheus.NewRegistry(),
		collections: make(map[string]*Collection),
	}
}

func (c *Collector) RegisterCollection(coll *Collection) {
	c.collections[coll.CollectionName] = coll
	for _, m := range coll.metrics {
		c.Registry.MustRegister(m.promMetric)
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

func (c *Collector) getCollection(subsystem string) *Collection {
	if collection, exists := c.collections[subsystem]; exists {
		return collection
	}
	return nil
}

func (c *Collector) Update(subsystem, metricName string, labelValues map[string]float64) {
	col := c.getCollection(subsystem)
	if col != nil {
		m := col.GetMetric(metricName)
		if m != nil {
			m.Update(labelValues)
		}
	}
}
