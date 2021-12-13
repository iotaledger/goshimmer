package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	analysisOutboundBytes    prometheus.Gauge
	gossipInboundPackets     prometheus.Gauge
	gossipOutboundPackets    prometheus.Gauge
	autopeeringInboundBytes  prometheus.Gauge
	autopeeringOutboundBytes prometheus.Gauge
)

func registerNetworkMetrics() {
	autopeeringInboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_autopeering_inbound_bytes",
		Help: "traffic_autopeering RX network traffic [bytes].",
	})
	autopeeringOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_autopeering_outbound_bytes",
		Help: "traffic_autopeering TX network traffic [bytes].",
	})
	gossipInboundPackets = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_gossip_inbound_packets",
		Help: "traffic_gossip RX network packets [number].",
	})
	gossipOutboundPackets = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_gossip_outbound_packets",
		Help: "traffic_gossip TX network packets [number].",
	})
	analysisOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "traffic_analysis_outbound_bytes",
		Help: "traffic_Analysis client TX network traffic [bytes].",
	})

	if deps.AutoPeeringConnMetric != nil {
		registry.MustRegister(autopeeringInboundBytes)
		registry.MustRegister(autopeeringOutboundBytes)
	}
	registry.MustRegister(analysisOutboundBytes)
	registry.MustRegister(gossipInboundPackets)
	registry.MustRegister(gossipOutboundPackets)

	addCollect(collectNetworkMetrics)
}

func collectNetworkMetrics() {
	if deps.AutoPeeringConnMetric != nil && deps.AutoPeeringConnMetric.UDPConn != nil {
		autopeeringInboundBytes.Set(float64(deps.AutoPeeringConnMetric.RXBytes()))
		autopeeringOutboundBytes.Set(float64(deps.AutoPeeringConnMetric.TXBytes()))
	}
	analysisOutboundBytes.Set(float64(metrics.AnalysisOutboundBytes()))
	gossipInboundPackets.Set(float64(metrics.GossipInboundPackets()))
	gossipOutboundPackets.Set(float64(metrics.GossipOutboundPackets()))
}
