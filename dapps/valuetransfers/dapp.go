package valuetransfers

import (
	"os"
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
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	// PluginName contains the human readable name of the plugin.
	PluginName = "ValueTransfers"

	// DefaultAverageNetworkDelay contains the default average time it takes for a network to propagate through gossip.
	DefaultAverageNetworkDelay = 5 * time.Second

	// CfgValueLayerSnapshotFile is the path to the snapshot file.
	CfgValueLayerSnapshotFile = "valueLayer.snapshot.file"

	// CfgValueLayerFCOBAverageNetworkDelay is the avg. network delay to use for FCoB rules
	CfgValueLayerFCOBAverageNetworkDelay = "valueLayer.fcob.averageNetworkDelay"
)

func init() {
	flag.String(CfgValueLayerSnapshotFile, "./snapshot.bin", "the path to the snapshot file")
	flag.Int(CfgValueLayerFCOBAverageNetworkDelay, 5, "the avg. network delay to use for FCoB rules")
}

var (
	// app is the "plugin" instance of the value-transfers application.
	app     *node.Plugin
	appOnce sync.Once

	// _tangle represents the value tangle that is used to express votes on value transactions.
	_tangle    *tangle.Tangle
	tangleOnce sync.Once

	// fcob contains the fcob consensus logic.
	fcob *consensus.FCOB

	// ledgerState represents the ledger state, that keeps track of the liked branches and offers an API to access funds.
	ledgerState *tangle.LedgerState

	// log holds a reference to the logger used by this app.
	log *logger.Logger

	tipManager     *tipmanager.TipManager
	tipManagerOnce sync.Once

	valueObjectFactory     *tangle.ValueObjectFactory
	valueObjectFactoryOnce sync.Once
)

// App gets the plugin instance.
func App() *node.Plugin {
	appOnce.Do(func() {
		app = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return app
}

// Tangle gets the tangle instance.
// tangle represents the value tangle that is used to express votes on value transactions.
func Tangle() *tangle.Tangle {
	tangleOnce.Do(func() {
		_tangle = tangle.New(database.Store())
	})
	return _tangle
}

// FCOB gets the fcob instance.
// fcob contains the fcob consensus logic.
func FCOB() *consensus.FCOB {
	return fcob
}

// LedgerState gets the ledgerState instance.
// ledgerState represents the ledger state, that keeps track of the liked branches and offers an API to access funds.
func LedgerState() *tangle.LedgerState {
	return ledgerState
}

func configure(_ *node.Plugin) {
	// configure logger
	log = logger.NewLogger(PluginName)

	// configure Tangle
	_tangle = Tangle()

	// configure LedgerState
	ledgerState = tangle.NewLedgerState(Tangle())

	// read snapshot file
	snapshotFilePath := config.Node().GetString(CfgValueLayerSnapshotFile)
	if len(snapshotFilePath) != 0 {
		snapshot := tangle.Snapshot{}
		f, err := os.Open(snapshotFilePath)
		if err != nil {
			log.Panic("can not open snapshot file:", err)
		}
		if _, err := snapshot.ReadFrom(f); err != nil {
			log.Panic("could not read snapshot file:", err)
		}
		_tangle.LoadSnapshot(snapshot)
		log.Infof("read snapshot from %s", snapshotFilePath)
	}

	_tangle.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Error(err)
	}))

	// initialize tip manager and value object factory
	tipManager = TipManager()
	valueObjectFactory = ValueObjectFactory()

	_tangle.Events.PayloadLiked.Attach(events.NewClosure(func(cachedPayload *payload.CachedPayload, cachedMetadata *tangle.CachedPayloadMetadata) {
		cachedMetadata.Release()
		cachedPayload.Consume(tipManager.AddTip)
	}))
	_tangle.Events.PayloadDisliked.Attach(events.NewClosure(func(cachedPayload *payload.CachedPayload, cachedMetadata *tangle.CachedPayloadMetadata) {
		cachedMetadata.Release()
		cachedPayload.Consume(tipManager.RemoveTip)
	}))

	// configure FCOB consensus rules
	cfgAvgNetworkDelay := config.Node().GetInt(CfgValueLayerFCOBAverageNetworkDelay)
	log.Infof("avg. network delay configured to %d seconds", cfgAvgNetworkDelay)
	fcob = consensus.NewFCOB(_tangle, time.Duration(cfgAvgNetworkDelay)*time.Second)
	fcob.Events.Vote.Attach(events.NewClosure(func(id string, initOpn vote.Opinion) {
		if err := voter.Vote(id, initOpn); err != nil {
			log.Warnf("FPC vote: %s", err)
		}
	}))
	fcob.Events.Error.Attach(events.NewClosure(func(err error) {
		log.Errorf("FCOB error: %s", err)
	}))

	// configure FPC + link to consensus
	configureFPC()
	voter.Events().Finalized.Attach(events.NewClosure(fcob.ProcessVoteResult))
	voter.Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		log.Infof("FPC finalized for transaction with id '%s' - final opinion: '%s'", ev.ID, ev.Opinion)
	}))
	voter.Events().Failed.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		log.Warnf("FPC failed for transaction with id '%s' - last opinion: '%s'", ev.ID, ev.Opinion)
	}))

	// register SignatureFilter in Parser
	messagelayer.MessageParser().AddMessageFilter(tangle.NewSignatureFilter())

	// subscribe to message-layer
	messagelayer.Tangle().Events.MessageSolid.Attach(events.NewClosure(onReceiveMessageFromMessageLayer))
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("ValueTangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		_tangle.Shutdown()
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

	_tangle.AttachPayload(valuePayload)
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
