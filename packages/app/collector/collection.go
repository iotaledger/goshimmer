package collector

import (
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/options"
)

type Collection struct {
	CollectionName string
	metrics        []*Metric
}

func NewCollection(name string, opts ...options.Option[Collection]) *Collection {
	return options.Apply(&Collection{
		CollectionName: name,
	}, opts)
}

// region Options ///////////////////////////////////////////////////////////////////////////////////////////////////////

func WithMetric(metric *Metric) options.Option[Collection] {
	return func(c *Collection) {
		c.metrics = append(c.metrics, metric)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func SingleValue[T constraints.Numeric](val T) map[string]float64 {
	return map[string]float64{
		"value": float64(val),
	}
}
