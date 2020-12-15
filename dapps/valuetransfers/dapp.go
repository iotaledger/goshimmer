package valuetransfers

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/consensus"
	valuepayload "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	valuetangle "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tipmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
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
	// ErrTransactionWasNotBookedInTime is returned if a transaction did not get booked
	// within the defined await time.
	ErrTransactionWasNotBookedInTime = errors.New("transaction could not be booked in time")

	// app is the "plugin" instance of the value-transfers application.
	app     *node.Plugin
	appOnce sync.Once

	// _tangle represents the value tangle that is used to express votes on value transactions.
	_tangle    *valuetangle.Tangle
	tangleOnce sync.Once

	// fcob contains the fcob consensus logic.
	fcob     *consensus.FCOB
	fcobOnce sync.Once

	// ledgerState represents the ledger state, that keeps track of the liked branches and offers an API to access funds.
	ledgerState *valuetangle.LedgerState

	// log holds a reference to the logger used by this app.
	log *logger.Logger

	tipManager     *tipmanager.TipManager
	tipManagerOnce sync.Once

	valueObjectFactory     *valuetangle.ValueObjectFactory
	valueObjectFactoryOnce sync.Once

	receiveMessageClosure *events.Closure
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
func Tangle() *valuetangle.Tangle {
	tangleOnce.Do(func() {
		_tangle = valuetangle.New(database.Store())
	})
	return _tangle
}

// FCOB gets the fcob instance.
// fcob contains the fcob consensus logic.
func FCOB() *consensus.FCOB {
	fcobOnce.Do(func() {
		// configure FCOB consensus rules
		cfgAvgNetworkDelay := config.Node().Int(CfgValueLayerFCOBAverageNetworkDelay)
		log.Infof("avg. network delay configured to %d seconds", cfgAvgNetworkDelay)
		fcob = consensus.NewFCOB(_tangle, time.Duration(cfgAvgNetworkDelay)*time.Second)
	})
	return fcob
}

// LedgerState gets the ledgerState instance.
// ledgerState represents the ledger state, that keeps track of the liked branches and offers an API to access funds.
func LedgerState() *valuetangle.LedgerState {
	return ledgerState
}

func configure(_ *node.Plugin) {
	// configure logger
	log = logger.NewLogger(PluginName)

	// configure Tangle
	_tangle = Tangle()

	// configure LedgerState
	ledgerState = valuetangle.NewLedgerState(Tangle())

	// read snapshot file
	snapshotFilePath := config.Node().String(CfgValueLayerSnapshotFile)
	if len(snapshotFilePath) != 0 {
		snapshot := valuetangle.Snapshot{}
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

	_tangle.Events.PayloadConfirmed.Attach(events.NewClosure(func(cachedPayloadEvent *valuetangle.CachedPayloadEvent) {
		cachedPayloadEvent.PayloadMetadata.Release()
		cachedPayloadEvent.Payload.Consume(tipManager.AddTip)
	}))

	// register SignatureFilter in Parser
	messagelayer.MessageParser().AddMessageFilter(valuetangle.NewSignatureFilter())

	// subscribe to message-layer
	receiveMessageClosure = events.NewClosure(onReceiveMessageFromMessageLayer)
	messagelayer.Tangle().Events.MessageSolid.Attach(receiveMessageClosure)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("ValueTangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		// TODO: make this better
		// stop listening to stuff from the message tangle. By the time we are here, gossip and autopeering have already
		// been shutdown, so no new incoming messages should appear.
		messagelayer.Tangle().Events.MessageSolid.Detach(receiveMessageClosure)
		// wait one network delay to be sure that all scheduled setPreferred are triggered in fcob. Otherwise, we would
		// try to access an already shutdown objectstorage in fcob.
		cfgAvgNetworkDelay := config.Node().Int(CfgValueLayerFCOBAverageNetworkDelay)
		time.Sleep(time.Duration(cfgAvgNetworkDelay) * time.Second)
		_tangle.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func onReceiveMessageFromMessageLayer(cachedMessageEvent *tangle.CachedMessageEvent) {
	defer cachedMessageEvent.Message.Release()
	defer cachedMessageEvent.MessageMetadata.Release()

	solidMessage := cachedMessageEvent.Message.Unwrap()
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
func ValueObjectFactory() *valuetangle.ValueObjectFactory {
	valueObjectFactoryOnce.Do(func() {
		valueObjectFactory = valuetangle.NewValueObjectFactory(Tangle(), TipManager())
	})
	return valueObjectFactory
}

// AwaitTransactionToBeBooked awaits maxAwait for the given transaction to get booked.
func AwaitTransactionToBeBooked(txID transaction.ID, maxAwait time.Duration) error {
	booked := make(chan struct{}, 1)
	// exit is used to let the caller exit if for whatever
	// reason the same transaction gets booked multiple times
	exit := make(chan struct{})
	defer close(exit)
	closure := events.NewClosure(func(cachedTransactionBookEvent *valuetangle.CachedTransactionBookEvent) {
		defer cachedTransactionBookEvent.Transaction.Release()
		defer cachedTransactionBookEvent.TransactionMetadata.Release()
		if cachedTransactionBookEvent.Transaction.Unwrap().ID() != txID {
			return
		}
		select {
		case booked <- struct{}{}:
		case <-exit:
		}
	})
	Tangle().Events.TransactionBooked.Attach(closure)
	defer Tangle().Events.TransactionBooked.Detach(closure)
	select {
	case <-time.After(maxAwait):
		return ErrTransactionWasNotBookedInTime
	case <-booked:
		return nil
	}
}
