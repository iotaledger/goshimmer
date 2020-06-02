package valuetransfers

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/consensus"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tipmanager"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	messageTangle "github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	// PluginName contains the human readable name of the plugin.
	PluginName = "ValueTransfers"

	// AverageNetworkDelay contains the average time it takes for a network to propagate through gossip.
	AverageNetworkDelay = 5 * time.Second
)

var (
	// App is the "plugin" instance of the value-transfers application.
	App = node.NewPlugin(PluginName, node.Enabled, configure, run)

	// Tangle represents the value tangle that is used to express votes on value transactions.
	Tangle *tangle.Tangle

	// FCOB contains the fcob consensus logic.
	FCOB *consensus.FCOB

	// LedgerState represents the ledger state, that keeps track of the liked branches and offers an API to access funds.
	LedgerState *tangle.LedgerState

	// log holds a reference to the logger used by this app.
	log *logger.Logger

	tipManager     *tipmanager.TipManager
	tipManagerOnce sync.Once

	valueObjectFactory     *tangle.ValueObjectFactory
	valueObjectFactoryOnce sync.Once
)

func configure(_ *node.Plugin) {
	// configure logger
	log = logger.NewLogger(PluginName)

	// configure Tangle
	Tangle = tangle.New(database.Store())
	Tangle.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Error(err)
	}))

	// initialize tip manager and value object factory
	tipManager = TipManager()
	valueObjectFactory = ValueObjectFactory()

	Tangle.Events.PayloadLiked.Attach(events.NewClosure(func(cachedPayload *payload.CachedPayload, cachedMetadata *tangle.CachedPayloadMetadata) {
		cachedMetadata.Release()
		cachedPayload.Consume(tipManager.AddTip)
	}))
	Tangle.Events.PayloadDisliked.Attach(events.NewClosure(func(cachedPayload *payload.CachedPayload, cachedMetadata *tangle.CachedPayloadMetadata) {
		cachedMetadata.Release()
		cachedPayload.Consume(tipManager.RemoveTip)
	}))

	// configure FCOB consensus rules
	FCOB = consensus.NewFCOB(Tangle, AverageNetworkDelay)
	FCOB.Events.Vote.Attach(events.NewClosure(func(id string, initOpn vote.Opinion) {
		if err := voter.Vote(id, initOpn); err != nil {
			log.Error(err)
		}
	}))
	FCOB.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Error(err)
	}))

	// configure FPC + link to consensus
	configureFPC()
	voter.Events().Finalized.Attach(events.NewClosure(FCOB.ProcessVoteResult))
	voter.Events().Failed.Attach(events.NewClosure(func(id string, lastOpinion vote.Opinion) {
		log.Errorf("FPC failed for transaction with id '%s' - last opinion: '%s'", id, lastOpinion)
	}))

	// subscribe to message-layer
	messagelayer.Tangle.Events.MessageSolid.Attach(events.NewClosure(onReceiveMessageFromMessageLayer))
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("ValueTangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		Tangle.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}

	runFPC()
}

func onReceiveMessageFromMessageLayer(cachedMessage *message.CachedMessage, cachedMessageMetadata *messageTangle.CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()

	solidMessage := cachedMessage.Unwrap()
	if solidMessage == nil {
		log.Debug("failed to unpack solid message from message layer")

		return
	}

	messagePayload := solidMessage.Payload()
	if messagePayload.Type() != valuepayload.Type {
		return
	}

	valuePayload, ok := messagePayload.(*valuepayload.Payload)
	if !ok {
		log.Debug("could not cast payload to value payload")

		return
	}

	Tangle.AttachPayload(valuePayload)
}

// TipManager returns the TipManager singleton.
func TipManager() *tipmanager.TipManager {
	tipManagerOnce.Do(func() {
		tipManager = tipmanager.New()
	})
	return tipManager
}

// ValueObjectFactory returns the ValueObjectFactory singleton.
func ValueObjectFactory() *tangle.ValueObjectFactory {
	valueObjectFactoryOnce.Do(func() {
		valueObjectFactory = tangle.NewValueObjectFactory(TipManager())
	})
	return valueObjectFactory
}
