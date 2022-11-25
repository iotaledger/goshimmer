package metricscollector

import "github.com/iotaledger/hive.go/core/generics/options"

type Collection struct {
	CollectionName string
	metrics        []*Metric
}

func NewCollection(name string) *Collection {
	return &Collection{
		CollectionName: name,
	}
}

// region Options ///////////////////////////////////////////////////////////////////////////////////////////////////////

func WithMetric(metric *Metric) options.Option[Collection] {
	return func(c *Collection) {
		c.metrics = append(c.metrics, metric)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
