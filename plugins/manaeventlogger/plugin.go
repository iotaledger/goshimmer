package manaeventlogger

import (
	"context"
	"encoding/csv"
	"os"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/packages/shutdown"
)

const (
	// PluginName is the name of the mana events logger plugin.
	PluginName = "ManaEventLogger"
)

var (
	// Plugin is the plugin instance of the manaeventlogger plugin.
	Plugin               *node.Plugin
	log                  *logger.Logger
	onPledgeEventClosure *events.Closure
	onRevokeEventClosure *events.Closure
	eventsBuffer         []mana.Event
	eventsBufferSize     int
	csvPath              string
	mu                   sync.Mutex
	csvMu                sync.Mutex
	checkBufferInterval  time.Duration
)

func init() {
	Plugin = node.NewPlugin(PluginName, nil, node.Disabled, configure, run)
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
	eventsBufferSize = Parameters.BufferSize
	csvPath = Parameters.CSV
	checkBufferInterval = Parameters.CheckBufferInterval
	onPledgeEventClosure = events.NewClosure(logPledge)
	onRevokeEventClosure = events.NewClosure(logRevoke)
	configureEvents()
}

func configureEvents() {
	mana.Events().Pledged.Attach(onPledgeEventClosure)
	mana.Events().Revoked.Attach(onRevokeEventClosure)
}

func logPledge(ev *mana.PledgedEvent) {
	eventsBuffer = append(eventsBuffer, ev)
}

func logRevoke(ev *mana.RevokedEvent) {
	eventsBuffer = append(eventsBuffer, ev)
}

func checkBuffer() {
	mu.Lock()
	defer mu.Unlock()
	if len(eventsBuffer) < eventsBufferSize {
		return
	}
	evs := make([]mana.Event, len(eventsBuffer))
	copy(evs, eventsBuffer)
	go func() {
		if err := writeEventsToCSV(evs); err != nil {
			log.Infof("error writing events to csv: %w", err)
		}
	}()
	eventsBuffer = nil
}

func writeEventsToCSV(evs []mana.Event) error {
	csvMu.Lock()
	defer csvMu.Unlock()
	if len(evs) == 0 {
		return nil
	}
	f, err := os.OpenFile(csvPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o660)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if fi.Size() == 0 {
		values := evs[0].ToPersistable().ToStringKeys()
		if err := w.Write(values); err != nil {
			log.Infof("error writing to csv: %w", err)
		}
	}

	for _, e := range evs {
		values := e.ToPersistable().ToStringValues()
		if err := w.Write(values); err != nil {
			log.Infof("error writing to csv: %w", err)
		}
	}

	return nil
}

func run(_ *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, func(ctx context.Context) {
		defer log.Infof("Stopping %s ... done", PluginName)
		ticker := time.NewTicker(checkBufferInterval)
		defer ticker.Stop()
	L:
		for {
			select {
			case <-ctx.Done():
				break L
			case <-ticker.C:
				checkBuffer()
			}
		}
		log.Infof("stopping %s", PluginName)
		mana.Events().Pledged.Detach(onPledgeEventClosure)
		mana.Events().Pledged.Detach(onRevokeEventClosure)
		if err := writeEventsToCSV(eventsBuffer); err != nil {
			log.Infof("error writing events to csv: %w", err)
		}
	}, shutdown.PriorityMana); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
