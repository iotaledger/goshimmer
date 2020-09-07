package dashboard

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var (
	visualizerWorkerCount     = 1
	visualizerWorkerQueueSize = 500
	visualizerWorkerPool      *workerpool.WorkerPool
)

// vertex defines a vertex in a DAG.
type vertex struct {
	ID        string `json:"id"`
	Parent1ID string `json:"parent1_id"`
	Parent2ID string `json:"parent2_id"`
	IsSolid   bool   `json:"is_solid"`
}

// tipinfo holds information about whether a given message is a tip or not.
type tipinfo struct {
	ID    string `json:"id"`
	IsTip bool   `json:"is_tip"`
}

func configureVisualizer() {
	visualizerWorkerPool = workerpool.New(func(task workerpool.Task) {

		switch x := task.Param(0).(type) {
		case *message.CachedMessage:
			sendVertex(x, task.Param(1).(*tangle.CachedMessageMetadata))
		case message.ID:
			sendTipInfo(x, task.Param(1).(bool))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))
}

func sendVertex(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()

	msg := cachedMessage.Unwrap()
	if msg == nil {
		return
	}
	broadcastWsMessage(&wsmsg{MsgTypeVertex, &vertex{
		ID:        msg.ID().String(),
		Parent1ID: msg.Parent1ID().String(),
		Parent2ID: msg.Parent2ID().String(),
		IsSolid:   cachedMessageMetadata.Unwrap().IsSolid(),
	}}, true)
}

func sendTipInfo(messageID message.ID, isTip bool) {
	broadcastWsMessage(&wsmsg{MsgTypeTipInfo, &tipinfo{
		ID:    messageID.String(),
		IsTip: isTip,
	}}, true)
}

func runVisualizer() {
	notifyNewMsg := events.NewClosure(func(cachedMsgEvent *tangle.CachedMessageEvent) {
		defer cachedMsgEvent.Message.Release()
		defer cachedMsgEvent.MessageMetadata.Release()
		_, ok := visualizerWorkerPool.TrySubmit(cachedMsgEvent.Message.Retain(), cachedMsgEvent.MessageMetadata.Retain())
		if !ok {
			cachedMsgEvent.Message.Release()
			cachedMsgEvent.MessageMetadata.Release()
		}
	})

	notifyNewTip := events.NewClosure(func(messageId message.ID) {
		visualizerWorkerPool.TrySubmit(messageId, true)
	})

	notifyDeletedTip := events.NewClosure(func(messageId message.ID) {
		visualizerWorkerPool.TrySubmit(messageId, false)
	})

	if err := daemon.BackgroundWorker("Dashboard[Visualizer]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle().Events.MessageAttached.Attach(notifyNewMsg)
		defer messagelayer.Tangle().Events.MessageAttached.Detach(notifyNewMsg)
		messagelayer.Tangle().Events.MessageSolid.Attach(notifyNewMsg)
		defer messagelayer.Tangle().Events.MessageSolid.Detach(notifyNewMsg)
		messagelayer.TipSelector().Events.TipAdded.Attach(notifyNewTip)
		defer messagelayer.TipSelector().Events.TipAdded.Detach(notifyNewTip)
		messagelayer.TipSelector().Events.TipRemoved.Attach(notifyDeletedTip)
		defer messagelayer.TipSelector().Events.TipRemoved.Detach(notifyDeletedTip)
		visualizerWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[Visualizer] ...")
		visualizerWorkerPool.Stop()
		log.Info("Stopping Dashboard[Visualizer] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
