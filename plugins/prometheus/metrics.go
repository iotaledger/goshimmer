package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/metrics"
)

func init() {
	if config.Node.GetBool(metrics.CfgMetricsLocal) {
		registerAutopeeringMetrics()
		registerDataMetrics()
		registerFPCMetrics()
		registerInfoMetrics()
		registerNetworkMetrics()
		registerProcessMetrics()
		registerTangleMetrics()
	}

	if config.Node.GetBool(metrics.CfgMetricsGlobal) {
		registerClientsMetrics()
	}
}
