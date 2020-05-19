package consensus

import (
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/labstack/gommon/log"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/vote"
)

// FCOB defines the "Fast Consensus of Barcelona" rules that are used to form the initial opinions of nodes. It uses a
// local modified based approach to reach approximate consensus within the network.
type FCOB struct {
	Events *FCOBEvents

	tangle              *tangle.Tangle
	averageNetworkDelay time.Duration
}

// NewFCOB is the constructor for an FCOB consensus instance. It automatically attaches to the passed in Tangle and
// calls the corresponding Events if it needs to trigger a vote.
func NewFCOB(tangle *tangle.Tangle, averageNetworkDelay time.Duration) (fcob *FCOB) {
	fcob = &FCOB{
		tangle:              tangle,
		averageNetworkDelay: averageNetworkDelay,
		Events: &FCOBEvents{
			Error: events.NewEvent(events.ErrorCaller),
		},
	}

	// setup behavior of package instances
	tangle.Events.TransactionBooked.Attach(events.NewClosure(fcob.onTransactionBooked))
	tangle.Events.Fork.Attach(events.NewClosure(fcob.onFork))

	return
}

// ProcessVoteResult allows an external voter to hand in the results of the voting process.
func (fcob *FCOB) ProcessVoteResult(id string, opinion vote.Opinion) {
	transactionID, err := transaction.IDFromBase58(id)
	if err != nil {
		fcob.Events.Error.Trigger(err)

		return
	}

	if _, err := fcob.tangle.SetTransactionPreferred(transactionID, opinion == vote.Like); err != nil {
		fcob.Events.Error.Trigger(err)
	}
}

func (fcob *FCOB) onTransactionBooked(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *tangle.CachedTransactionMetadata, decisionPending bool) {
	defer cachedTransaction.Release()

	cachedTransactionMetadata.Consume(func(transactionMetadata *tangle.TransactionMetadata) {
		if transactionMetadata.Conflicting() {
			// abort if the previous consumers where finalized already
			if !decisionPending {
				return
			}

			fcob.Events.Vote.Trigger(transactionMetadata.BranchID().String(), vote.Dislike)

			return
		}

		fcob.scheduleSetPreferred(cachedTransactionMetadata.Retain())
	})
}

func (fcob *FCOB) scheduleSetPreferred(cachedTransactionMetadata *tangle.CachedTransactionMetadata) {
	time.AfterFunc(fcob.averageNetworkDelay, func() {
		cachedTransactionMetadata.Consume(func(transactionMetadata *tangle.TransactionMetadata) {
			if transactionMetadata.Conflicting() {
				return
			}

			modified, err := fcob.tangle.SetTransactionPreferred(transactionMetadata.ID(), true)
			if err != nil {
				log.Error(err)

				return
			}

			if modified {
				fcob.scheduleSetFinalized(cachedTransactionMetadata.Retain())
			}
		})
	})
}

func (fcob *FCOB) scheduleSetFinalized(cachedTransactionMetadata *tangle.CachedTransactionMetadata) {
	time.AfterFunc(fcob.averageNetworkDelay, func() {
		cachedTransactionMetadata.Consume(func(transactionMetadata *tangle.TransactionMetadata) {
			if transactionMetadata.Conflicting() {
				return
			}

			transactionMetadata.SetFinalized(true)
		})
	})
}

// TODO: clarify what we do here
func (fcob *FCOB) onFork(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *tangle.CachedTransactionMetadata) {
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()

	transactionMetadata := cachedTransactionMetadata.Unwrap()
	if transactionMetadata == nil {
		return
	}

	switch transactionMetadata.Preferred() {
	case true:
		fcob.Events.Vote.Trigger(transactionMetadata.ID().String(), vote.Like)
	case false:
		fcob.Events.Vote.Trigger(transactionMetadata.ID().String(), vote.Dislike)
	}
}

// FCOBEvents acts as a dictionary for events of an FCOB instance.
type FCOBEvents struct {
	Error *events.Event
	Vote  *events.Event
}
