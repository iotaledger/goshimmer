package prometheus

import (
	"fmt"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

var (
	lastCommitment                   *prometheus.GaugeVec
	numberOfSeenOtherCommitments     prometheus.Gauge
	missingCommitmentsRequested      prometheus.Gauge
	numberMissingCommitmentsReceived prometheus.Gauge
	acceptedBlksOfEpoch              *prometheus.GaugeVec
	numberOfBlockRemoved             *prometheus.GaugeVec
)

func registerEpochCommitmentMetrics() {
	lastCommitment = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "last_commitment",
		Help: "Info about last commitment",
	}, []string{
		"epoch", "commitment",
	})

	numberOfSeenOtherCommitments = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "number_of_seen_other_commitments",
		Help: "Number of commitments seen by the node that are different than its own",
	})
	missingCommitmentsRequested = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "missing_commitments_requested",
		Help: "Number of missing commitments requested by the node",
	})

	numberMissingCommitmentsReceived = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "missing_commitments_received",
		Help: "Number of missing commitments received by the node",
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
	registry.MustRegister(lastCommitment)
	registry.MustRegister(numberOfSeenOtherCommitments)
	registry.MustRegister(missingCommitmentsRequested)
	registry.MustRegister(numberMissingCommitmentsReceived)
	registry.MustRegister(numberOfBlockRemoved)
	registry.MustRegister(acceptedBlksOfEpoch)

	addCollect(collectEpochCommitmentMetrics)
	addCollect(collectRemovedBlockMetrics)
	addCollect(collectNumOfAcceptedBlkMetrics)
}

func collectEpochCommitmentMetrics() {
	commitment := metrics.LastCommittedEpoch()
	lastCommitment.WithLabelValues(strconv.Itoa(int(commitment.Index())), commitment.ID().Base58()).Set(float64(commitment.Index()))

	otherCommitments := metrics.NumberOfSeenOtherCommitments()
	numberOfSeenOtherCommitments.Set(float64(otherCommitments))

	missingCommitments := metrics.MissingCommitmentsRequested()
	missingCommitmentsRequested.Set(float64(missingCommitments))

	missingCommitmentsReceived := metrics.MissingCommitmentsReceived()
	numberMissingCommitmentsReceived.Set(float64(missingCommitmentsReceived))
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
