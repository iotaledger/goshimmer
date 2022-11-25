package metricscollector

import "github.com/iotaledger/hive.go/core/generics/options"

type MetricType uint8

const (
	Gauge MetricType = iota
	GaugeVex
	Counter
)

type Metric struct {
	Name string
	Type MetricType
}

func NewMetric(name string, metricType MetricType, opts ...options.Option[Metric]) *Metric {
	return options.Apply(&Metric{
		Name: name,
		Type: metricType,
	}, opts)

}

// region Options ///////////////////////////////////////////////////////////////////////////////////////////////////////
