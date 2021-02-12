package messagelayer

import (
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"golang.org/x/xerrors"
)

const (
	// PluginName defines the plugin name.
	PluginName = "MessageLayer"
)

var (
	// ErrMessageWasNotBookedInTime is returned if a message did not get booked
	// within the defined await time.
	ErrMessageWasNotBookedInTime = errors.New("message could not be booked in time")
)

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
			tangle.WithoutOpinionFormer(true),
		)
	})

	return tangleInstance
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
	Tangle().Setup()

	// Bypass the Booker
	Tangle().Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
			messageMetadata.SetBooked(true)
			Tangle().Booker.Events.MessageBooked.Trigger(messageID)
		})
	}))

	Tangle().Events.Error.Attach(events.NewClosure(func(err error) {
		log.Error(err)
	}))
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
		Tangle().Storage.Message(msgID).Consume(func(message *tangle.Message) {
			tx := message.Payload().(*ledgerstate.Transaction)
			if tx.ID() == txID {
				return
			}
		})
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
