package metrics

import (
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"go.uber.org/atomic"
)

var (
	// tangle layer
	messageStorage                    atomic.Uint64
	messageMetadataStorage            atomic.Uint64
	approverStorage                   atomic.Uint64
	missingMessageStorage             atomic.Uint64
	attachmentStorage                 atomic.Uint64
	markerIndexBranchIDMappingStorage atomic.Uint64
	individuallyMappedMessageStorage  atomic.Uint64
	sequenceSupportersStorage         atomic.Uint64
	branchSupportersStorage           atomic.Uint64
	statementStorage                  atomic.Uint64
	branchWeightStorage               atomic.Uint64
	markerMessageMappingStorage       atomic.Uint64

	// utxodag/ledgerstate
	transactionStorage          atomic.Uint64
	transactionMetadataStorage  atomic.Uint64
	outputStorage               atomic.Uint64
	outputMetadataStorage       atomic.Uint64
	consumerStorage             atomic.Uint64
	addressOutputMappingStorage atomic.Uint64

	// fcob
	fcobOpinionStorage          atomic.Uint64
	fcobTimestampOpinionStorage atomic.Uint64
	fcobMessageMetadataStorage  atomic.Uint64

	// branchdag
	branchStorage         atomic.Uint64
	childBranchStorage    atomic.Uint64
	conflictStorage       atomic.Uint64
	conflictMemberStorage atomic.Uint64

	// markers
	sequenceStore             atomic.Uint64
	sequenceAliasMappingStore atomic.Uint64

	//mana
	accessManaStore    atomic.Uint64
	consensusManaStore atomic.Uint64
)

func MessageStorageSize() uint64 {
	return messageStorage.Load()
}
func MessageMetadataStorageSize() uint64 {
	return messageMetadataStorage.Load()
}
func ApproverStorageSize() uint64 {
	return approverStorage.Load()
}
func MissingMessageStorageSize() uint64 {
	return missingMessageStorage.Load()
}
func AttachmentStorageSize() uint64 {
	return attachmentStorage.Load()
}
func MarkerIndexBranchIDMappingStorageSize() uint64 {
	return markerIndexBranchIDMappingStorage.Load()
}
func IndividuallyMappedMessageStorageSize() uint64 {
	return individuallyMappedMessageStorage.Load()
}
func SequenceSupportersStorageSize() uint64 {
	return sequenceSupportersStorage.Load()
}
func BranchSupportersStorageSize() uint64 {
	return branchSupportersStorage.Load()
}
func StatementStorageSize() uint64 {
	return statementStorage.Load()
}
func BranchWeightStorageSize() uint64 {
	return branchWeightStorage.Load()
}
func MarkerMessageMappingStorageSize() uint64 {
	return markerMessageMappingStorage.Load()
}
func TransactionStorageSize() uint64 {
	return transactionStorage.Load()
}
func TransactionMetadataStorageSize() uint64 {
	return transactionMetadataStorage.Load()
}
func OutputStorageSize() uint64 {
	return outputStorage.Load()
}
func OutputMetadataStorageSize() uint64 {
	return outputMetadataStorage.Load()
}
func ConsumerStorageSize() uint64 {
	return consumerStorage.Load()
}
func AddressOutputMappingStorageSize() uint64 {
	return addressOutputMappingStorage.Load()
}
func FcobOpinionStorageSize() uint64 {
	return fcobOpinionStorage.Load()
}
func FcobTimestampOpinionStorageSize() uint64 {
	return fcobTimestampOpinionStorage.Load()
}
func FcobMessageMetadataStorageSize() uint64 {
	return fcobMessageMetadataStorage.Load()
}
func BranchStorageSize() uint64 {
	return branchStorage.Load()
}
func ChildBranchStorageSize() uint64 {
	return childBranchStorage.Load()
}
func ConflictStorageSize() uint64 {
	return conflictStorage.Load()
}
func ConflictMemberStorageSize() uint64 {
	return conflictMemberStorage.Load()
}
func SequenceStoreSize() uint64 {
	return sequenceStore.Load()
}
func SequenceAliasMappingStoreSize() uint64 {
	return sequenceAliasMappingStore.Load()
}
func AccessManaStoreSize() uint64 {
	return accessManaStore.Load()
}
func ConsensusManaStoreSize() uint64 {
	return consensusManaStore.Load()
}

func measureStorageSizes() {
	messageStorage.Store(messagelayer.Tangle().Storage.MessagetStorageSize())
	messageMetadataStorage.Store(messagelayer.Tangle().Storage.MessageMetadataStorageSize())
	approverStorage.Store(messagelayer.Tangle().Storage.ApproverStorageSize())
	missingMessageStorage.Store(messagelayer.Tangle().Storage.MissingMessageStorageSize())
	attachmentStorage.Store(messagelayer.Tangle().Storage.AttachmentStorageSize())
	markerIndexBranchIDMappingStorage.Store(messagelayer.Tangle().Storage.MarkerIndexBranchIDMappingStorageSize())
	individuallyMappedMessageStorage.Store(messagelayer.Tangle().Storage.IndividuallyMappedMessageStorageSize())
	sequenceSupportersStorage.Store(messagelayer.Tangle().Storage.SequenceSupportersStorageSize())
	branchSupportersStorage.Store(messagelayer.Tangle().Storage.BranchSupportersStorageSize())
	statementStorage.Store(messagelayer.Tangle().Storage.StatementStorageSize())
	branchWeightStorage.Store(messagelayer.Tangle().Storage.BranchWeightStorageSize())
	markerMessageMappingStorage.Store(messagelayer.Tangle().Storage.MarkerMessageMappingStorageSize())

	transactionStorage.Store(messagelayer.Tangle().LedgerState.UTXODAG.TransactionStorageSize())
	transactionMetadataStorage.Store(messagelayer.Tangle().LedgerState.UTXODAG.TransactionMetadataStorageSize())
	outputStorage.Store(messagelayer.Tangle().LedgerState.UTXODAG.OutputStorageSize())
	outputMetadataStorage.Store(messagelayer.Tangle().LedgerState.UTXODAG.OutputMetadataStorageSize())
	consumerStorage.Store(messagelayer.Tangle().LedgerState.UTXODAG.ConsumerStorageSize())
	addressOutputMappingStorage.Store(messagelayer.Tangle().LedgerState.UTXODAG.AddressOutputMappingStorageSize())

	fcobOpinionStorage.Store(messagelayer.ConsensusMechanism().Storage.OpinionStorageSize())
	fcobTimestampOpinionStorage.Store(messagelayer.ConsensusMechanism().Storage.TimestampOpinionStorageSize())
	fcobMessageMetadataStorage.Store(messagelayer.ConsensusMechanism().Storage.MessageMetadataStorageSize())

	branchStorage.Store(messagelayer.Tangle().LedgerState.BranchDAG.BranchStorageSize())
	childBranchStorage.Store(messagelayer.Tangle().LedgerState.BranchDAG.ChildBranchStorageSize())
	conflictStorage.Store(messagelayer.Tangle().LedgerState.BranchDAG.ConflictStorageSize())
	conflictMemberStorage.Store(messagelayer.Tangle().LedgerState.BranchDAG.ConflictMemberStorageSize())

	sequenceStore.Store(messagelayer.Tangle().Booker.MarkersManager.SequenceStoreSize())
	sequenceAliasMappingStore.Store(messagelayer.Tangle().Booker.MarkersManager.SequenceAliasMappingStoreSize())

	accessManaStore.Store(messagelayer.AccessManaVectorStorageSize())
	consensusManaStore.Store(messagelayer.ConsensusManaVectorStorageSize())
}
