package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	fpcInboundBytes          prometheus.Gauge
	fpcOutboundBytes         prometheus.Gauge
	analysisOutboundBytes    prometheus.Gauge
	gossipInboundBytes       prometheus.Gauge
	gossipOutboundBytes      prometheus.Gauge
	autopeeringInboundBytes  prometheus.Gauge
	autopeeringOutboundBytes prometheus.Gauge
)

func registerNetworkMetrics() {
	fpcInboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_fpc_inbound_bytes",
		Help: "FPC RX network traffic [bytes].",
	})
	fpcOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_fpc_outbound_bytes",
		Help: "FPC TX network traffic [bytes].",
	})
	autopeeringInboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_autopeering_inbound_bytes",
		Help: "traffic_autopeering RX network traffic [bytes].",
	})
	autopeeringOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_autopeering_outbound_bytes",
		Help: "traffic_autopeering TX network traffic [bytes].",
	})
	gossipInboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_gossip_inbound_bytes",
		Help: "traffic_gossip RX network traffic [bytes].",
	})
	gossipOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_gossip_outbound_bytes",
		Help: "traffic_gossip TX network traffic [bytes].",
	})
	analysisOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_analysis_outbound_bytes",
		Help: "traffic_Analysis client TX network traffic [bytes].",
	})

	registry.MustRegister(fpcInboundBytes)
	registry.MustRegister(fpcOutboundBytes)
	registry.MustRegister(analysisOutboundBytes)
	registry.MustRegister(autopeeringInboundBytes)
	registry.MustRegister(autopeeringOutboundBytes)
	registry.MustRegister(gossipInboundBytes)
	registry.MustRegister(gossipOutboundBytes)

	addCollect(collectNetworkMetrics)
}

func collectNetworkMetrics() {
	fpcInboundBytes.Set(float64(metrics.FPCInboundBytes()))
	fpcOutboundBytes.Set(float64(metrics.FPCOutboundBytes()))
	analysisOutboundBytes.Set(float64(metrics.AnalysisOutboundBytes()))
	autopeeringInboundBytes.Set(float64(autopeering.Conn.RXBytes()))
	autopeeringOutboundBytes.Set(float64(autopeering.Conn.TXBytes()))
	gossipInboundBytes.Set(float64(metrics.GossipInboundBytes()))
	gossipOutboundBytes.Set(float64(metrics.GossipOutboundBytes()))
}
