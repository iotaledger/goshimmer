package prometheus

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	workerpools *prometheus.GaugeVec
)

func registerWorkerpoolMetrics() {
	workerpools = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "workerpools_load",
			Help: "Info about workerpools load",
		},
		[]string{
			"name",
		},
	)

	registry.MustRegister(workerpools)

	addCollect(collectWorkerpoolMetrics)
}

func collectWorkerpoolMetrics() {
	name, load := gossip.Manager().WorkerPoolStatus()
	workerpools.WithLabelValues(
		name,
	).Set(float64(load))

	name, load = messagelayer.Tangle().SolidifierWorkerPoolStatus()
	workerpools.WithLabelValues(
		name,
	).Set(float64(load))

	name, load = messagelayer.Tangle().StoreMessageWorkerPoolStatus()
	workerpools.WithLabelValues(
		name,
	).Set(float64(load))

	if messagelayer.MessageParser().MessageSignatureFilter != nil {
		name, load = messagelayer.MessageParser().MessageSignatureFilter.WorkerPoolStatus()
		workerpools.WithLabelValues(
			name,
		).Set(float64(load))
	}

	if valuetransfers.SignatureFilter != nil {
		name, load = valuetransfers.SignatureFilter.WorkerPoolStatus()
		workerpools.WithLabelValues(
			name,
		).Set(float64(load))
	}

	if messagelayer.MessageParser().RecentlySeenBytesFilter != nil {
		name, load = messagelayer.MessageParser().RecentlySeenBytesFilter.WorkerPoolStatus()
		workerpools.WithLabelValues(
			name,
		).Set(float64(load))
	}

	if messagelayer.MessageParser().PowFilter != nil {
		name, load = messagelayer.MessageParser().PowFilter.WorkerPoolStatus()
		workerpools.WithLabelValues(
			name,
		).Set(float64(load))
	}
}
