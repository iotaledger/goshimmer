package collector

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Collector is responsible for creation and collection of metrics for the prometheus.
type Collector struct {
	Registry *prometheus.Registry
}

// New creates an instance of Manger, creates new prometheus registry for the protocol metrics collection.
func New() *Collector {
	return &Collector{
		Registry: prometheus.NewRegistry(),
	}
}

func (c *Collector) RegisterCollection(collection *Collection) {

}

func (c *Collector) Collect() {

}

func (c *Collector) registerLocalMetrics() {

}
