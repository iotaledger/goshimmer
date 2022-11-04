package prometheus

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	lastCommittedEpoch   prometheus.Gauge
	numberOfBlockRemoved *prometheus.GaugeVec
)

func registerEpochCommittmentMetrics() {
	lastCommittedEpoch = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "last_committed_ei",
		Help: "Info about last committed epoch Index",
	})

	numberOfBlockRemoved = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "blk_removed_from_epoch",
			Help: "Info about orphaned block removed from an epoch",
		}, []string{
			"epoch",
		})

	registry.MustRegister(lastCommittedEpoch)
	registry.MustRegister(numberOfBlockRemoved)

	// TODO: uncomment when commitments work
	//addCollect(collectEpochCommittmentMetrics)
	addCollect(collectRemovedBlockMetrics)
}

func collectEpochCommittmentMetrics() {
	commitment := metrics.LastCommittedEpoch()
	lastCommittedEpoch.Set(float64(commitment.Index()))
}

func collectRemovedBlockMetrics() {
	fmt.Println(">>>>>> removed block from epoch triggered!!!")
	blockCounts := metrics.RemovedBlocksOfEpoch()
	for ei, count := range blockCounts {
		numberOfBlockRemoved.WithLabelValues(ei.String()).Set(float64(count))
	}
}
