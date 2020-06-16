package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	messagesPerSecond        prometheus.Gauge
	fpcInboundBytes          prometheus.Gauge
	fpcOutboundBytes         prometheus.Gauge
	analysisOutboundBytes    prometheus.Gauge
	autopeeringInboundBytes  prometheus.Gauge
	autopeeringOutboundBytes prometheus.Gauge
)

func init() {
	messagesPerSecond = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "iota_messages_per_second",
		Help: "Number of messages per second.",
	})

	// Network
	fpcInboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "FPC_inbound_bytes",
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
	analysisOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "analysis_outbound_bytes",
		Help: "Analysis client TX network traffic [bytes].",
	})

	registry.MustRegister(messagesPerSecond)
	registry.MustRegister(fpcInboundBytes)
	registry.MustRegister(fpcOutboundBytes)
	registry.MustRegister(analysisOutboundBytes)
	registry.MustRegister(autopeeringInboundBytes)
	registry.MustRegister(autopeeringOutboundBytes)

	addCollect(collectLocalMetrics)
}

func collectLocalMetrics() {
	messagesPerSecond.Set(float64(metrics.ReceivedMessagesPerSecond()))
	fpcInboundBytes.Set(float64(metrics.FPCInboundBytes()))
	fpcOutboundBytes.Set(float64(metrics.FPCOutboundBytes()))
	analysisOutboundBytes.Set(float64(metrics.AnalysisOutboundBytes()))
	autopeeringInboundBytes.Set(float64(autopeering.Conn.RXBytes()))
	autopeeringOutboundBytes.Set(float64(autopeering.Conn.TXBytes()))
}
