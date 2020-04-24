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

var visualizerWorkerCount = 1
var visualizerWorkerQueueSize = 50
var visualizerWorkerPool *workerpool.WorkerPool

type vertex struct {
	ID       string `json:"id"`
	TrunkId  string `json:"trunk_id"`
	BranchId string `json:"branch_id"`
	IsSolid  bool   `json:"is_solid"`
}

type tipinfo struct {
	ID    string `json:"id"`
	IsTip bool   `json:"is_tip"`
}

func configureVisualizer() {
	visualizerWorkerPool = workerpool.New(func(task workerpool.Task) {

		switch x := task.Param(0).(type) {
		case *message.CachedMessage:
			sendVertex(x, task.Param(1).(*tangle.CachedMessageMetadata))
		case message.Id:
			sendTipInfo(x, task.Param(1).(bool))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))
}

func sendVertex(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
	defer cachedMessage.Release()
	defer cachedMessageMetadata.Release()

	msg := cachedMessage.Unwrap()
	sendToAllWSClient(&wsmsg{MsgTypeVertex, &vertex{
		ID:       msg.Id().String(),
		TrunkId:  msg.TrunkId().String(),
		BranchId: msg.BranchId().String(),
		IsSolid:  cachedMessageMetadata.Unwrap().IsSolid(),
	}}, true)
}

func sendTipInfo(messageId message.Id, isTip bool) {
	sendToAllWSClient(&wsmsg{MsgTypeTipInfo, &tipinfo{
		ID:    messageId.String(),
		IsTip: isTip,
	}}, true)
}

func runVisualizer() {
	notifyNewMsg := events.NewClosure(func(message *message.CachedMessage, metadata *tangle.CachedMessageMetadata) {
		defer message.Release()
		defer metadata.Release()
		visualizerWorkerPool.TrySubmit(message.Retain(), metadata.Retain())
	})

	notifyNewTip := events.NewClosure(func(messageId message.Id) {
		visualizerWorkerPool.TrySubmit(messageId, true)
	})

	notifyDeletedTip := events.NewClosure(func(messageId message.Id) {
		visualizerWorkerPool.TrySubmit(messageId, false)
	})

	daemon.BackgroundWorker("Dashboard[Visualizer]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle.Events.MessageAttached.Attach(notifyNewMsg)
		messagelayer.Tangle.Events.MessageSolid.Attach(notifyNewMsg)
		messagelayer.TipSelector.Events.TipAdded.Attach(notifyNewTip)
		messagelayer.TipSelector.Events.TipRemoved.Attach(notifyDeletedTip)
		visualizerWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[Visualizer] ...")
		messagelayer.Tangle.Events.MessageAttached.Detach(notifyNewMsg)
		messagelayer.TipSelector.Events.TipAdded.Detach(notifyNewTip)
		messagelayer.TipSelector.Events.TipRemoved.Detach(notifyDeletedTip)
		visualizerWorkerPool.Stop()
		log.Info("Stopping Dashboard[Visualizer] ... done")
	}, shutdown.PriorityDashboard)
}
