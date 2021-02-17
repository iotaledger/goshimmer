package consensus

import (
	"time"

	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
)

// FCOB defines the "Fast Consensus of Barcelona" rules that are used to form the initial opinions of nodes. It uses a
// local modifier based approach to reach approximate consensus within the network by waiting 1 network delay before
// setting a transaction to preferred (if it didnt see a conflict) and another network delay to set it to finalized (if
// it still didn't see a conflict).
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
			Vote:  events.NewEvent(voteEvent),
		},
	}

	// setup behavior of package instances
	tangle.Events.TransactionBooked.Attach(events.NewClosure(fcob.onTransactionBooked))
	tangle.Events.Fork.Attach(events.NewClosure(fcob.onFork))

	return
}

// ProcessVoteResult allows an external voter to hand in the results of the voting process.
func (fcob *FCOB) ProcessVoteResult(ev *vote.OpinionEvent) {
	if ev.Ctx.Type == vote.ConflictType {
		transactionID, err := transaction.IDFromBase58(ev.ID)
		if err != nil {
			fcob.Events.Error.Trigger(err)

			return
		}

		if _, err := fcob.tangle.SetTransactionPreferred(transactionID, ev.Opinion == opinion.Like); err != nil {
			fcob.Events.Error.Trigger(err)
		}

		if _, err := fcob.tangle.SetTransactionFinalized(transactionID); err != nil {
			fcob.Events.Error.Trigger(err)
		}
	}
}

// onTransactionBooked analyzes the transaction that was booked by the Tangle and initiates the FCOB rules if it is not
// conflicting. If it is conflicting and a decision is still pending we trigger a voting process.
func (fcob *FCOB) onTransactionBooked(cachedTransactionBookEvent *tangle.CachedTransactionBookEvent) {
	defer cachedTransactionBookEvent.Transaction.Release()

	cachedTransactionBookEvent.TransactionMetadata.Consume(func(transactionMetadata *tangle.TransactionMetadata) {
		if transactionMetadata.Conflicting() {
			// abort if the previous consumers where finalized already
			if !cachedTransactionBookEvent.Pending {
				return
			}

			fcob.Events.Vote.Trigger(transactionMetadata.BranchID().String(), opinion.Dislike)

			return
		}

		fcob.scheduleSetPreferred(cachedTransactionBookEvent.TransactionMetadata.Retain())
	})
}

// scheduleSetPreferred schedules the setPreferred logic after 1 network delay.
func (fcob *FCOB) scheduleSetPreferred(cachedTransactionMetadata *tangle.CachedTransactionMetadata) {
	if fcob.averageNetworkDelay == 0 {
		fcob.setPreferred(cachedTransactionMetadata)
	} else {
		time.AfterFunc(fcob.averageNetworkDelay, func() {
			fcob.setPreferred(cachedTransactionMetadata)
		})
	}
}

// setPreferred sets the Transaction to preferred if it is not conflicting.
func (fcob *FCOB) setPreferred(cachedTransactionMetadata *tangle.CachedTransactionMetadata) {
	cachedTransactionMetadata.Consume(func(transactionMetadata *tangle.TransactionMetadata) {
		if transactionMetadata.Conflicting() {
			return
		}

		modified, err := fcob.tangle.SetTransactionPreferred(transactionMetadata.ID(), true)
		if err != nil {
			fcob.Events.Error.Trigger(err)

			return
		}

		if modified {
			fcob.scheduleSetFinalized(cachedTransactionMetadata.Retain())
		}
	})
}

// scheduleSetFinalized schedules the setFinalized logic after 2 network delays.
// Note: it is 2 network delays because this function gets triggered at the end of the first delay that sets a
// Transaction to preferred (see setPreferred).
func (fcob *FCOB) scheduleSetFinalized(cachedTransactionMetadata *tangle.CachedTransactionMetadata) {
	if fcob.averageNetworkDelay == 0 {
		fcob.setFinalized(cachedTransactionMetadata)
	} else {
		time.AfterFunc(fcob.averageNetworkDelay, func() {
			fcob.setFinalized(cachedTransactionMetadata)
		})
	}
}

// setFinalized sets the Transaction to finalized if it is not conflicting.
func (fcob *FCOB) setFinalized(cachedTransactionMetadata *tangle.CachedTransactionMetadata) {
	cachedTransactionMetadata.Consume(func(transactionMetadata *tangle.TransactionMetadata) {
		if transactionMetadata.Conflicting() {
			return
		}

		if _, err := fcob.tangle.SetTransactionFinalized(transactionMetadata.ID()); err != nil {
			fcob.Events.Error.Trigger(err)
		}
	})
}

// onFork triggers a voting process whenever a Transaction gets forked into a new Branch. The initial opinion is derived
// from the preferred flag that was set using the FCOB rule.
func (fcob *FCOB) onFork(forkEvent *tangle.ForkEvent) {
	defer forkEvent.Transaction.Release()
	defer forkEvent.TransactionMetadata.Release()
	defer forkEvent.Branch.Release()

	transactionMetadata := forkEvent.TransactionMetadata.Unwrap()
	if transactionMetadata == nil {
		return
	}

	switch transactionMetadata.Preferred() {
	case true:
		fcob.Events.Vote.Trigger(transactionMetadata.ID().String(), opinion.Like)
	case false:
		fcob.Events.Vote.Trigger(transactionMetadata.ID().String(), opinion.Dislike)
	}
}

// FCOBEvents acts as a dictionary for events of an FCOB instance.
type FCOBEvents struct {
	// Error gets called when FCOB faces an error.
	Error *events.Event

	// Vote gets called when FCOB needs to vote on a transaction.
	Vote *events.Event
}

func voteEvent(handler interface{}, params ...interface{}) {
	handler.(func(id string, initOpn opinion.Opinion))(params[0].(string), params[1].(opinion.Opinion))
}
