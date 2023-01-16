package metrics

import "github.com/iotaledger/goshimmer/packages/app/collector"

const (
	rateSetterNamespace = "ratesetter"

	estimate   = "estimate"
	ownRate    = "own_rate"
	bufferSize = "buffer_size"
)

var RateSetterMetrics = collector.NewCollection(rateSetterNamespace,
	collector.WithMetric(collector.NewMetric(estimate,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Current rate estimate for the node."),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(deps.BlockIssuer.RateSetter.Estimate())
		}),
	)),
	collector.WithMetric(collector.NewMetric(ownRate,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Current rate of the node."),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(deps.BlockIssuer.RateSetter.Rate())
		}),
	)),
	collector.WithMetric(collector.NewMetric(bufferSize,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Current size of the rate setter buffer."),
		collector.WithCollectFunc(func() map[string]float64 {
			return collector.SingleValue(deps.BlockIssuer.RateSetter.Size())
		}),
	)),
)
