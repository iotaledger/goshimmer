package metricscollector

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Collector is responsible for creation and collection of metrics for the prometheus.
type Collector struct {
	Registry *prometheus.Registry
}

// NewCollector creates an instance of Manger.
func NewCollector() *Collector {
	return &Collector{
		Registry: prometheus.NewRegistry(),
	}
}

func (c *Collector) RegisterCollection(m *Collection) {

}

func (c *Collector) Collect() {

}

func (c *Collector) registerLocalMetrics() {

}
