package statusscreen

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	name            = "Statusscreen"
	repaintInterval = 1 * time.Second
)

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

var (
	stdLogMsgClosure   = events.NewClosure(stdLogMsg)
	storeLogMsgClosure = events.NewClosure(storeLogMsg)
)

func init() {
	// use standard go logger by default
	logger.Events.AnyMsg.Attach(stdLogMsgClosure)
}

func configure(*node.Plugin) {
	if !isTerminal() {
		return
	}

	// store any log message for display
	logger.Events.AnyMsg.Attach(storeLogMsgClosure)

	log = logger.NewLogger(name)
	configureTview()
}

func run(*node.Plugin) {
	if !isTerminal() {
		return
	}

	stopped := make(chan struct{})
	if err := daemon.BackgroundWorker(name+" Refresher", func(shutdown <-chan struct{}) {
		for {
			select {
			case <-time.After(repaintInterval):
				app.QueueUpdateDraw(func() {})
			case <-shutdown:
				logger.Events.AnyMsg.Detach(storeLogMsgClosure)
				app.Stop()
				return
			case <-stopped:
				return
			}
		}
	}, shutdown.ShutdownPriorityStatusScreen); err != nil {
		log.Errorf("Failed to start as daemon: %s", err)
		return
	}

	if err := daemon.BackgroundWorker(name+" App", func(<-chan struct{}) {
		defer close(stopped)

		// switch logging to status screen
		logger.Events.AnyMsg.Detach(stdLogMsgClosure)
		defer logger.Events.AnyMsg.Attach(stdLogMsgClosure)

		if err := app.SetRoot(frame, true).SetFocus(frame).Run(); err != nil {
			log.Errorf("Error running application: %s", err)
		}
	}, shutdown.ShutdownPriorityStatusScreen); err != nil {
		log.Errorf("Failed to start as daemon: %s", err)
		close(stopped)
	}
}
