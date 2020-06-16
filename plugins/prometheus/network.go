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

func init() {
	fpcInboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fpc_inbound_bytes",
		Help: "FPC RX network traffic [bytes].",
	})
	fpcOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "fpc_outbound_bytes",
		Help: "FPC TX network traffic [bytes].",
	})
	autopeeringInboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "autopeering_inbound_bytes",
		Help: "autopeering RX network traffic [bytes].",
	})
	autopeeringOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "autopeering_outbound_bytes",
		Help: "autopeering TX network traffic [bytes].",
	})
	gossipInboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gossip_inbound_bytes",
		Help: "gossip RX network traffic [bytes].",
	})
	gossipOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "gossip_outbound_bytes",
		Help: "gossip TX network traffic [bytes].",
	})
	analysisOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "analysis_outbound_bytes",
		Help: "Analysis client TX network traffic [bytes].",
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
