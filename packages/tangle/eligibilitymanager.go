package tangle

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// EligibilityEvents contain all events that can be triggered by the manager
type EligibilityEvents struct {
	// MessageEligible is triggered when message eligible flag is set to true
	MessageEligible *events.Event
	Error           *events.Event
}

// EligibilityManager manages when tips become eligible and handle the persistence of the status
type EligibilityManager struct {
	Events *EligibilityEvents

	tangle       *Tangle
	storageMutex sync.Mutex
}

// NewEligibilityManager creates a new eligibility manager ready to trigger events
func NewEligibilityManager(tangle *Tangle) (eligibilityManager *EligibilityManager) {
	eligibilityManager = &EligibilityManager{
		Events: &EligibilityEvents{
			MessageEligible: events.NewEvent(MessageIDCaller),
			Error:           events.NewEvent(events.ErrorCaller),
		},

		tangle: tangle,
	}
	return
}

// checkEligibility checks if message cn be set to eligible. If yes, it set eligibility flag and triggers message eligible event.
// If not it stores transactions dependencies to the object storage
func (e *EligibilityManager) checkEligibility(messageID MessageID) error {
	cachedMsg := e.tangle.Storage.Message(messageID)
	defer cachedMsg.Release()

	message := cachedMsg.Unwrap()
	payloadType := message.Payload().Type()
	// data messages are eligible right after solidification
	if payloadType != ledgerstate.TransactionType {
		e.makeEligible(messageID)
		return nil
	}

	tx := message.Payload().(*ledgerstate.Transaction)
	pendingDependencies, err := e.obtainPendingDependencies(tx)
	if err != nil {
		return errors.Errorf("failed to get pending transaction's dependencies: %w", err)
	}
	// no pending dependencies left
	if len(pendingDependencies) == 0 {
		e.makeEligible(messageID)
		return nil
		// All dependencies are approved by the message
	} else if NewUtils(e.tangle).AllTransactionsDirectlyApprovedByMessages(pendingDependencies, messageID) {
		e.makeEligible(messageID)
		return nil
	}

	for dependencyTxID := range pendingDependencies {
		txID := tx.ID()
		err = e.storeMissingDependencies(&txID, &dependencyTxID)
		if err != nil {
			return errors.Errorf("failed to add missing dependencies: %w", err)
		}
	}
	return nil
}

// storeMissingDependencies adds missing dependencies to the object storage
// or append if already exist append dependent txID to the existing one
func (e *EligibilityManager) storeMissingDependencies(dependentTxID *ledgerstate.TransactionID, dependencyTxID *ledgerstate.TransactionID) error {
	e.storageMutex.Lock()
	defer e.storageMutex.Unlock()

	storage := e.tangle.Storage
	if cachedDependencies := storage.UnconfirmedTransactionDependencies(*dependencyTxID); cachedDependencies != nil {
		cachedDependencies.Consume(func(unconfirmedTxDependency *UnconfirmedTxDependency) {
			unconfirmedTxDependency.AddDependency(*dependentTxID)
		})
		return nil
	}
	txDependency := NewUnconfirmedTxDependency(dependencyTxID)
	txDependency.AddDependency(dependentTxID)
	cachedDependency := storage.StoreUnconfirmedTransactionDependencies(txDependency)
	if cachedDependency == nil {
		return errors.Errorf("failed to store dependency, txID: %s", dependentTxID)
	}
	cachedDependency.Release()

	return nil
}

func (e *EligibilityManager) makeEligible(messageID MessageID) {
	e.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		messageMetadata.SetEligible(true)
	})
	e.Events.MessageEligible.Trigger(messageID)
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (e *EligibilityManager) Setup() {
	e.tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID MessageID) {
		if err := e.checkEligibility(messageID); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to check eligibility of message %s. %w", messageID.Base58(), err))
		}
	}))

	e.tangle.LedgerState.UTXODAG.Events().TransactionConfirmed.Attach(events.NewClosure(func(transactionID *ledgerstate.TransactionID) {
		if err := e.updateEligibilityAfterDependencyConfirmation(transactionID); err != nil {
			e.Events.Error.Trigger(errors.Errorf("failed to update eligibility after tx %s was confirmed. %w", transactionID.Base58(), err))
		}
	}))
}

// updateEligibilityAfterDependencyConfirmation triggered on transaction confirmation event
// remove unconfirmed dependency from object storage and checks if any of dependent transactions
// can be set to eligible
func (e *EligibilityManager) updateEligibilityAfterDependencyConfirmation(dependencyTxID *ledgerstate.TransactionID) error {
	dependentTxs := make([]*ledgerstate.Transaction, 0)
	// get all txID dependent on this transactionID
	if cachedDependencies := e.tangle.Storage.UnconfirmedTransactionDependencies(*dependencyTxID); cachedDependencies != nil {
		// tx that need to be checked for the eligibility after one of dependency has been confirmed
		cachedDependencies.Consume(func(unconfirmedTxDependency *UnconfirmedTxDependency) {
			// get tx from storage
			for txID := range unconfirmedTxDependency.dependentTxIDs {
				e.tangle.LedgerState.Transaction(txID).Consume(func(tx *ledgerstate.Transaction) {
					dependentTxs = append(dependentTxs, tx)
				})
			}
		})
		e.tangle.Storage.deleteUnconfirmedTxDependencies(*dependencyTxID)
	}

	for _, dependentTx := range dependentTxs {
		pendingDependencies, err := e.obtainPendingDependencies(dependentTx)
		if err != nil {
			return errors.Errorf("failed to get pending transaction's dependencies: %w", err)
		}
		if len(pendingDependencies) == 0 {
			e.makeAttachmentsEligible(dependentTx)
		}
	}
	return nil
}

// obtainPendingDependencies return all transaction ids that are still pending confirmation that tx depends upon
func (e *EligibilityManager) obtainPendingDependencies(tx *ledgerstate.Transaction) (ledgerstate.TransactionIDs, error) {
	pendingDependencies := make(ledgerstate.TransactionIDs)
	for _, input := range tx.Essence().Inputs() {
		outputID := input.(*ledgerstate.UTXOInput).ReferencedOutputID()
		txID := outputID.TransactionID()
		state, err := e.tangle.LedgerState.UTXODAG.InclusionState(txID)
		if err != nil {
			return nil, errors.Errorf("failed to get inclusion state: %w", err)
		}
		if state != ledgerstate.Confirmed {
			pendingDependencies[txID] = struct{}{}
		}
	}
	return pendingDependencies, nil
}

// makeAttachmentsEligible set eligibility to true for all transaction's attachments if all dependencies has been confirmed
func (e *EligibilityManager) makeAttachmentsEligible(dependentTx *ledgerstate.Transaction) {
	// set eligible all tx dependent attachments
	messageIDs := e.tangle.Storage.AttachmentMessageIDs(dependentTx.ID())
	for _, messageID := range messageIDs {
		e.makeEligible(messageID)
	}
}

// UnconfirmedTxDependenciesFromObjectStorage restores an UnconfirmedTxDependency object that was stored in the ObjectStorage.
func UnconfirmedTxDependenciesFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = UnconfirmedTxDependencyFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse UnconfirmedTxDependencyFromByte from bytes: %w", err)
		return
	}

	return
}

// UnconfirmedTxDependencyFromBytes unmarshals an UnconfirmedTxDependency from bytes.
func UnconfirmedTxDependencyFromBytes(bytes []byte) (unconfirmedTxDependency *UnconfirmedTxDependency,
	consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)

	txID, err := ledgerstate.TransactionIDFromMarshalUtil(marshalUtil)
	if err != nil {
		err = errors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
		return
	}
	txDependencies, err := ledgerstate.TransactionIDsFromMarshalUtil(marshalUtil)
	if err != nil {
		err = errors.Errorf("failed to parse TransactionIDs from MarshalUtil: %w", err)
		return
	}

	unconfirmedTxDependency = &UnconfirmedTxDependency{
		dependencyTxID: txID,
		dependentTxIDs: txDependencies,
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}
