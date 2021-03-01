package messagelayer

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
	"golang.org/x/xerrors"
)

const (
	// PluginName defines the plugin name.
	PluginName = "MessageLayer"
	// DefaultAverageNetworkDelay contains the default average time it takes for a network to propagate through gossip.
	DefaultAverageNetworkDelay = 5 * time.Second

	// CfgMessageLayerSnapshotFile is the path to the snapshot file.
	CfgMessageLayerSnapshotFile = "messageLayer.snapshot.file"

	// CfgMessageLayerFCOBAverageNetworkDelay is the avg. network delay to use for FCoB rules
	CfgMessageLayerFCOBAverageNetworkDelay = "messageLayer.fcob.averageNetworkDelay"

	// CfgTangleWidth is the width of the Tangle.
	CfgTangleWidth = "messageLayer.tangleWidth"
)

var (
	// ErrMessageWasNotBookedInTime is returned if a message did not get booked
	// within the defined await time.
	ErrMessageWasNotBookedInTime = errors.New("message could not be booked in time")
)

func init() {
	flag.String(CfgMessageLayerSnapshotFile, "./snapshot.bin", "the path to the snapshot file")
	flag.Int(CfgMessageLayerFCOBAverageNetworkDelay, 5, "the avg. network delay to use for FCoB rules")
	flag.Int(CfgTangleWidth, 0, "the width of the Tangle")
}

var (
	// plugin is the plugin instance of the message layer plugin.
	plugin         *node.Plugin
	pluginOnce     sync.Once
	tangleInstance *tangle.Tangle
	tangleOnce     sync.Once
	log            *logger.Logger
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	pluginOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

// Tangle gets the tangle instance.
func Tangle() *tangle.Tangle {
	tangleOnce.Do(func() {
		tangleInstance = tangle.New(
			tangle.Store(database.Store()),
			tangle.Identity(local.GetInstance().LocalIdentity()),
			tangle.TangleWidth(config.Node().Int(CfgTangleWidth)),
		)
	})

	return tangleInstance
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
	Tangle().Setup()

	Tangle().Events.Error.Attach(events.NewClosure(func(err error) {
		log.Error(err)
	}))

	// read snapshot file
	snapshotFilePath := config.Node().String(CfgMessageLayerSnapshotFile)
	if len(snapshotFilePath) != 0 {
		snapshot := ledgerstate.Snapshot{}
		f, err := os.Open(snapshotFilePath)
		if err != nil {
			log.Panic("can not open snapshot file:", err)
		}
		if _, err := snapshot.ReadFrom(f); err != nil {
			log.Panic("could not read snapshot file:", err)
		}
		Tangle().LedgerState.LoadSnapshot(snapshot)
		log.Infof("read snapshot from %s", snapshotFilePath)
	}

	avgNetworkDelay := config.Node().Int(CfgMessageLayerFCOBAverageNetworkDelay)
	tangle.LikedThreshold = (time.Duration(avgNetworkDelay) * time.Second)
	tangle.LocallyFinalizedThreshold = (time.Duration(avgNetworkDelay*2) * time.Second)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("Tangle", func(shutdownSignal <-chan struct{}) {
		<-shutdownSignal
		Tangle().Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

// AwaitMessageToBeBooked awaits maxAwait for the given message to get booked.
func AwaitMessageToBeBooked(f func() (*tangle.Message, error), txID ledgerstate.TransactionID, maxAwait time.Duration) (*tangle.Message, error) {
	// first subscribe to the transaction booked event
	booked := make(chan struct{}, 1)
	// exit is used to let the caller exit if for whatever
	// reason the same transaction gets booked multiple times
	exit := make(chan struct{})
	defer close(exit)

	closure := events.NewClosure(func(msgID tangle.MessageID) {
		match := false
		Tangle().Storage.Message(msgID).Consume(func(message *tangle.Message) {
			if message.Payload().Type() == ledgerstate.TransactionType {
				tx := message.Payload().(*ledgerstate.Transaction)
				if tx.ID() == txID {
					match = true
					return
				}
			}
		})
		if !match {
			return
		}
		select {
		case booked <- struct{}{}:
		case <-exit:
		}
	})
	Tangle().Booker.Events.MessageBooked.Attach(closure)
	defer Tangle().Booker.Events.MessageBooked.Detach(closure)

	// then issue the message with the tx

	// channel to receive the result of issuance
	issueResult := make(chan struct {
		msg *tangle.Message
		err error
	}, 1)

	go func() {
		msg, err := f()
		issueResult <- struct {
			msg *tangle.Message
			err error
		}{msg: msg, err: err}
	}()

	// wait on issuance
	result := <-issueResult

	if result.err != nil || result.msg == nil {
		return nil, xerrors.Errorf("Failed to issue transaction %s: %w", txID.String(), result.err)
	}

	select {
	case <-time.After(maxAwait):
		return nil, ErrMessageWasNotBookedInTime
	case <-booked:
		return result.msg, nil
	}
}
