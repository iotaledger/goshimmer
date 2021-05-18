package prometheus

import (
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/database"
)

var (
	dbSize prometheus.Gauge

	// tangle layer
	messageStorage                    prometheus.Gauge
	messageMetadataStorage            prometheus.Gauge
	approverStorage                   prometheus.Gauge
	missingMessageStorage             prometheus.Gauge
	attachmentStorage                 prometheus.Gauge
	markerIndexBranchIDMappingStorage prometheus.Gauge
	individuallyMappedMessageStorage  prometheus.Gauge
	sequenceSupportersStorage         prometheus.Gauge
	branchSupportersStorage           prometheus.Gauge
	statementStorage                  prometheus.Gauge
	branchWeightStorage               prometheus.Gauge
	markerMessageMappingStorage       prometheus.Gauge

	// utxodag/ledgerstate
	transactionStorage          prometheus.Gauge
	transactionMetadataStorage  prometheus.Gauge
	outputStorage               prometheus.Gauge
	outputMetadataStorage       prometheus.Gauge
	consumerStorage             prometheus.Gauge
	addressOutputMappingStorage prometheus.Gauge

	// fcob
	fcobOpinionStorage          prometheus.Gauge
	fcobTimestampOpinionStorage prometheus.Gauge
	fcobMessageMetadataStorage  prometheus.Gauge

	// branchdag
	branchStorage         prometheus.Gauge
	childBranchStorage    prometheus.Gauge
	conflictStorage       prometheus.Gauge
	conflictMemberStorage prometheus.Gauge

	// markers
	sequenceStore             prometheus.Gauge
	sequenceAliasMappingStore prometheus.Gauge

	//mana
	accessManaStore    prometheus.Gauge
	consensusManaStore prometheus.Gauge
)

func registerDBMetrics() {
	dbSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_size_bytes",
			Help: "DB size in bytes.",
		},
	)

	messageStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_messageStorage",
			Help: "messageStorage size in bytes.",
		},
	)
	messageMetadataStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_messageMetadataStorage",
			Help: "messageMetadataStorage size in bytes.",
		},
	)
	approverStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_approverStorage",
			Help: "approverStorage size in bytes.",
		},
	)
	missingMessageStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_missingMessageStorage",
			Help: "missingMessageStorage size in bytes.",
		},
	)
	attachmentStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_attachmentStorage",
			Help: "attachmentStorage size in bytes.",
		},
	)
	markerIndexBranchIDMappingStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_markerIndexBranchIDMappingStorage",
			Help: "markerIndexBranchIDMappingStorage size in bytes.",
		},
	)
	individuallyMappedMessageStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_individuallyMappedMessageStorage",
			Help: "individuallyMappedMessageStorage size in bytes.",
		},
	)
	sequenceSupportersStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_sequenceSupportersStorage",
			Help: "sequenceSupportersStorage size in bytes.",
		},
	)
	branchSupportersStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_branchSupportersStorage",
			Help: "branchSupportersStorage size in bytes.",
		},
	)
	statementStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_statementStorage",
			Help: "statementStorage size in bytes.",
		},
	)
	branchWeightStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_branchWeightStorage",
			Help: "branchWeightStorage size in bytes.",
		},
	)
	markerMessageMappingStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_markerMessageMappingStorage",
			Help: "markerMessageMappingStorage size in bytes.",
		},
	)

	transactionStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_transactionStorage",
			Help: "transactionStorage size in bytes.",
		},
	)
	transactionMetadataStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_transactionMetadataStorage",
			Help: "transactionMetadataStorage size in bytes.",
		},
	)
	outputStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_outputStorage",
			Help: "outputStorage size in bytes.",
		},
	)
	outputMetadataStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_outputMetadataStorage",
			Help: "outputMetadataStorage size in bytes.",
		},
	)
	consumerStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_consumerStorage",
			Help: "consumerStorage size in bytes.",
		},
	)
	addressOutputMappingStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_addressOutputMappingStorage",
			Help: "addressOutputMappingStorage size in bytes.",
		},
	)

	fcobOpinionStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_fcob_opinionStorage",
			Help: "fcobOpinionStorage size in bytes.",
		},
	)
	fcobTimestampOpinionStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_fcob_timestampOpinionStorage",
			Help: "fcobTimestampOpinionStorage size in bytes.",
		},
	)
	fcobMessageMetadataStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_fcob_messageMetadataStorage",
			Help: "fcobMessageMetadataStorage size in bytes.",
		},
	)

	branchStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_branchStorage",
			Help: "branchStorage size in bytes.",
		},
	)
	childBranchStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_childBranchStorage",
			Help: "childBranchStorage size in bytes.",
		},
	)
	conflictStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_conflictStorage",
			Help: "conflictStorage size in bytes.",
		},
	)
	conflictMemberStorage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_conflictMemberStorage",
			Help: "conflictMemberStorage size in bytes.",
		},
	)

	sequenceStore = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_sequenceStore",
			Help: "sequenceStore size in bytes.",
		},
	)
	sequenceAliasMappingStore = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_sequenceAliasMappingStore",
			Help: "sequenceAliasMappingStore size in bytes.",
		},
	)

	accessManaStore = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_accessManaStore",
			Help: "accessManaStore size in bytes.",
		},
	)

	consensusManaStore = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_consensusManaStore",
			Help: "consensusManaStore size in bytes.",
		},
	)

	registry.MustRegister(dbSize)

	registry.MustRegister(messageStorage)
	registry.MustRegister(messageMetadataStorage)
	registry.MustRegister(approverStorage)
	registry.MustRegister(missingMessageStorage)
	registry.MustRegister(attachmentStorage)
	registry.MustRegister(markerIndexBranchIDMappingStorage)
	registry.MustRegister(individuallyMappedMessageStorage)
	registry.MustRegister(sequenceSupportersStorage)
	registry.MustRegister(branchSupportersStorage)
	registry.MustRegister(statementStorage)
	registry.MustRegister(branchWeightStorage)
	registry.MustRegister(markerMessageMappingStorage)

	registry.MustRegister(transactionStorage)
	registry.MustRegister(transactionMetadataStorage)
	registry.MustRegister(outputStorage)
	registry.MustRegister(outputMetadataStorage)
	registry.MustRegister(consumerStorage)
	registry.MustRegister(addressOutputMappingStorage)

	registry.MustRegister(fcobOpinionStorage)
	registry.MustRegister(fcobTimestampOpinionStorage)
	registry.MustRegister(fcobMessageMetadataStorage)

	registry.MustRegister(branchStorage)
	registry.MustRegister(childBranchStorage)
	registry.MustRegister(conflictStorage)
	registry.MustRegister(conflictMemberStorage)

	registry.MustRegister(sequenceStore)
	registry.MustRegister(sequenceAliasMappingStore)

	registry.MustRegister(accessManaStore)
	registry.MustRegister(consensusManaStore)

	addCollect(collectDBSize)
}

func collectDBSize() {
	size, err := directorySize(config.Node().String(database.CfgDatabaseDir))
	if err == nil {
		dbSize.Set(float64(size))
	}

	messageStorage.Set(float64(metrics.MessageStorageSize()))
	messageMetadataStorage.Set(float64(metrics.MessageMetadataStorageSize()))
	approverStorage.Set(float64(metrics.ApproverStorageSize()))
	missingMessageStorage.Set(float64(metrics.MissingMessageStorageSize()))
	attachmentStorage.Set(float64(metrics.AttachmentStorageSize()))
	markerIndexBranchIDMappingStorage.Set(float64(metrics.MarkerIndexBranchIDMappingStorageSize()))
	individuallyMappedMessageStorage.Set(float64(metrics.IndividuallyMappedMessageStorageSize()))
	sequenceSupportersStorage.Set(float64(metrics.SequenceSupportersStorageSize()))
	branchSupportersStorage.Set(float64(metrics.BranchSupportersStorageSize()))
	statementStorage.Set(float64(metrics.StatementStorageSize()))
	branchWeightStorage.Set(float64(metrics.BranchWeightStorageSize()))
	markerMessageMappingStorage.Set(float64(metrics.MarkerMessageMappingStorageSize()))

	transactionStorage.Set(float64(metrics.TransactionStorageSize()))
	transactionMetadataStorage.Set(float64(metrics.TransactionMetadataStorageSize()))
	outputStorage.Set(float64(metrics.OutputStorageSize()))
	outputMetadataStorage.Set(float64(metrics.OutputMetadataStorageSize()))
	consumerStorage.Set(float64(metrics.ConsumerStorageSize()))
	addressOutputMappingStorage.Set(float64(metrics.AddressOutputMappingStorageSize()))

	fcobOpinionStorage.Set(float64(metrics.FcobOpinionStorageSize()))
	fcobTimestampOpinionStorage.Set(float64(metrics.FcobTimestampOpinionStorageSize()))
	fcobMessageMetadataStorage.Set(float64(metrics.FcobMessageMetadataStorageSize()))

	branchStorage.Set(float64(metrics.BranchStorageSize()))
	childBranchStorage.Set(float64(metrics.ChildBranchStorageSize()))
	conflictStorage.Set(float64(metrics.ConflictStorageSize()))
	conflictMemberStorage.Set(float64(metrics.ConflictMemberStorageSize()))

	sequenceStore.Set(float64(metrics.SequenceStoreSize()))
	sequenceAliasMappingStore.Set(float64(metrics.SequenceAliasMappingStoreSize()))

	accessManaStore.Set(float64(metrics.AccessManaStoreSize()))
	consensusManaStore.Set(float64(metrics.ConsensusManaStoreSize()))
}

func directorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
