package prometheus

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/metrics"
)

var (
	lastCommittedEpoch   prometheus.Gauge
	acceptedBlksOfEpoch  *prometheus.GaugeVec
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

	acceptedBlksOfEpoch = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "accepted_blks_of_epoch",
			Help: "Number of accepted blocks in an epoch",
		}, []string{
			"epoch",
		})

	registry.MustRegister(lastCommittedEpoch)
	registry.MustRegister(numberOfBlockRemoved)
	registry.MustRegister(acceptedBlksOfEpoch)

	// TODO: uncomment when commitments work
	//addCollect(collectEpochCommittmentMetrics)
	addCollect(collectRemovedBlockMetrics)
	addCollect(collectNumOfAcceptedBlkMetrics)
}

func collectEpochCommittmentMetrics() {
	commitment := metrics.LastCommittedEpoch()
	lastCommittedEpoch.Set(float64(commitment.Index()))
}

func collectRemovedBlockMetrics() {
	blockCounts := metrics.RemovedBlocksOfEpoch()
	for ei, count := range blockCounts {
		eiStr := fmt.Sprint(uint64(ei))
		numberOfBlockRemoved.WithLabelValues(eiStr).Set(float64(count))
	}
}

func collectNumOfAcceptedBlkMetrics() {
	ei, num := metrics.BlocksOfEpoch()
	eiStr := fmt.Sprint(uint64(ei))
	acceptedBlksOfEpoch.WithLabelValues(eiStr).Set(float64(num))
}
