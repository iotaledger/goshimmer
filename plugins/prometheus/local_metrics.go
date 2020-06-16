package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	messagesPerSecond     prometheus.Gauge
	fpcInboundBytes       prometheus.Gauge
	fpcOutboundBytes      prometheus.Gauge
	analysisOutboundBytes prometheus.Gauge
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
	analysisOutboundBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "analysis_outbound_bytes",
		Help: "Analysis client TX network traffic [bytes].",
	})

	registry.MustRegister(messagesPerSecond)
	registry.MustRegister(fpcInboundBytes)
	registry.MustRegister(fpcOutboundBytes)
	registry.MustRegister(analysisOutboundBytes)

	addCollect(collectLocalMetrics)
}

func collectLocalMetrics() {
	messagesPerSecond.Set(float64(metrics.ReceivedMessagesPerSecond()))
	fpcInboundBytes.Set(float64(metrics.FPCInboundBytes()))
	fpcOutboundBytes.Set(float64(metrics.FPCOutboundBytes()))
	analysisOutboundBytes.Set(float64(metrics.AnalysisOutboundBytes()))
}
