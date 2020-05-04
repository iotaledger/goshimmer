package valuetransfers

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/ledgerstate"
	valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/utxodag"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	messageTangle "github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	// PluginName contains the human readable name of the plugin.
	PluginName = "ValueTransfers"

	// AverageNetworkDelay contains the average time it takes for a network to propagate through gossip.
	AverageNetworkDelay = 6 * time.Second
)

var (
	// App is the "plugin" instance of the value-transfers application.
	App = node.NewPlugin(PluginName, node.Enabled, configure, run)

	// Tangle represents the value tangle that is used to express votes on value transactions.
	Tangle *tangle.Tangle

	// UTXODAG represents the flow of funds that is derived from the value tangle.
	UTXODAG *utxodag.UTXODAG

	// LedgerState represents the ledger state, that keeps track of the liked branches and offers an API to access funds.
	LedgerState *ledgerstate.LedgerState

	// log holds a reference to the logger used by this app.
	log *logger.Logger
)

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)

	log.Debug("configuring ValueTransfers")

	// create instances
	Tangle = tangle.New(database.GetBadgerInstance())
	UTXODAG = utxodag.New(database.GetBadgerInstance(), Tangle)

	// subscribe to message-layer
	messagelayer.Tangle.Events.MessageSolid.Attach(events.NewClosure(onReceiveMessageFromMessageLayer))

	// setup behavior of package instances
	Tangle.Events.PayloadSolid.Attach(events.NewClosure(UTXODAG.ProcessSolidPayload))
	UTXODAG.Events.TransactionBooked.Attach(events.NewClosure(onTransactionBooked))
	UTXODAG.Events.Fork.Attach(events.NewClosure(onFork))

	configureFPC()
	// TODO: DECIDE WHAT WE SHOULD DO IF FPC FAILS
	// voter.Events().Failed.Attach(events.NewClosure(panic))
	voter.Events().Finalized.Attach(events.NewClosure(func(id string, opinion vote.Opinion) {
		branchID, err := branchmanager.BranchIDFromBase58(id)
		if err != nil {
			log.Error(err)

			return
		}

		switch opinion {
		case vote.Like:
			UTXODAG.BranchManager().SetBranchPreferred(branchID, true)
		case vote.Dislike:
			UTXODAG.BranchManager().SetBranchPreferred(branchID, false)
		}
	}))
}

func run(*node.Plugin) {
	_ = daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		Tangle.Shutdown()
		UTXODAG.Shutdown()
	}, shutdown.PriorityTangle)

	runFPC()
}

func onReceiveMessageFromMessageLayer(cachedMessage *message.CachedMessage, cachedMessageMetadata *messageTangle.CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()

	solidMessage := cachedMessage.Unwrap()
	if solidMessage == nil {
		// TODO: LOG ERROR?

		return
	}

	messagePayload := solidMessage.Payload()
	if messagePayload.Type() != valuepayload.Type {
		// TODO: LOG ERROR?

		return
	}

	valuePayload, ok := messagePayload.(*valuepayload.Payload)
	if !ok {
		// TODO: LOG ERROR?

		return
	}

	Tangle.AttachPayload(valuePayload)
}

func onTransactionBooked(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *utxodag.CachedTransactionMetadata, cachedBranch *branchmanager.CachedBranch, conflictingInputs []transaction.OutputID, previousConsumersForked bool) {
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()
	defer cachedBranch.Release()

	if len(conflictingInputs) >= 1 {
		// abort if the previous consumers where finalized already
		if !previousConsumersForked {
			return
		}

		branch := cachedBranch.Unwrap()
		if branch == nil {
			log.Error("failed to unpack branch")

			return
		}

		err := voter.Vote(branch.ID().String(), vote.Dislike)
		if err != nil {
			log.Error(err)
		}

		return
	}

	// If the transaction is not conflicting, then we apply the fcob rule (we finalize after 2 network delays).
	// Note: We do not set a liked flag after 1 network delay because that can be derived.
	cachedTransactionMetadata.Retain()
	time.AfterFunc(2*AverageNetworkDelay, func() {
		defer cachedTransactionMetadata.Release()

		transactionMetadata := cachedTransactionMetadata.Unwrap()
		if transactionMetadata == nil {
			return
		}

		if transactionMetadata.BranchID() != branchmanager.NewBranchID(transactionMetadata.ID()) {
			return
		}

		transactionMetadata.SetFinalized(true)
	})
}

func onFork(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *utxodag.CachedTransactionMetadata, cachedBranch *branchmanager.CachedBranch, conflictingInputs []transaction.OutputID) {
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()
	defer cachedBranch.Release()

	transactionMetadata := cachedTransactionMetadata.Unwrap()
	if transactionMetadata == nil {
		return
	}

	branch := cachedBranch.Unwrap()
	if branch == nil {
		return
	}

	if time.Since(transactionMetadata.SoldificationTime()) < AverageNetworkDelay {
		return
	}

	if _, err := UTXODAG.BranchManager().SetBranchPreferred(branch.ID(), true); err != nil {
		log.Error(err)
	}
}
