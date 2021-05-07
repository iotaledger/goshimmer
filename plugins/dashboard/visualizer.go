package dashboard

import (
	"net/http"
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"

	"github.com/labstack/echo"
)

var (
	visualizerWorkerCount     = 1
	visualizerWorkerQueueSize = 500
	visualizerWorkerPool      *workerpool.WorkerPool

	msgHistoryMutex    sync.RWMutex
	msgOpinionFormed   map[string]bool
	msgHistory         []*tangle.Message
	maxMsgHistorySize  = 1000
	numHistoryToRemove = 100
)

// vertex defines a vertex in a DAG.
type vertex struct {
	ID              string   `json:"id"`
	StrongParentIDs []string `json:"strongParentIDs"`
	WeakParentIDs   []string `json:"weakParentIDs"`
	IsOpinionFormed bool     `json:"is_confirmed"`
	IsTx            bool     `json:"is_tx"`
}

// tipinfo holds information about whether a given message is a tip or not.
type tipinfo struct {
	ID    string `json:"id"`
	IsTip bool   `json:"is_tip"`
}

// history holds a set of vertices in a DAG.
type history struct {
	Vertices []vertex `json:"vertices"`
}

func configureVisualizer() {
	visualizerWorkerPool = workerpool.New(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *tangle.Message:
			sendVertex(x, task.Param(1).(bool))
		case tangle.TipType:
			sendTipInfo(task.Param(1).(tangle.MessageID), task.Param(2).(bool))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))

	// configure msgHistory, msgSolid
	msgOpinionFormed = make(map[string]bool, maxMsgHistorySize)
	msgHistory = make([]*tangle.Message, 0, maxMsgHistorySize)
}

func sendVertex(msg *tangle.Message, confirmed bool) {
	broadcastWsMessage(&wsmsg{MsgTypeVertex, &vertex{
		ID:              msg.ID().Base58(),
		StrongParentIDs: msg.StrongParents().ToStrings(),
		WeakParentIDs:   msg.WeakParents().ToStrings(),
		IsOpinionFormed: confirmed,
		IsTx:            msg.Payload().Type() == ledgerstate.TransactionType,
	}}, true)
}

func sendTipInfo(messageID tangle.MessageID, isTip bool) {
	broadcastWsMessage(&wsmsg{MsgTypeTipInfo, &tipinfo{
		ID:    messageID.Base58(),
		IsTip: isTip,
	}}, true)
}

func runVisualizer() {
	notifyNewMsg := events.NewClosure(func(messageID tangle.MessageID) {
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
			messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
				confirmed := false

				p := message.Payload()
				if p.Type() == ledgerstate.TransactionType {
					txID := p.(*ledgerstate.Transaction).ID()
					txInclusionState, _ := messagelayer.Tangle().LedgerState.TransactionInclusionState(txID)
					confirmed = txInclusionState == ledgerstate.Confirmed
				} else {
					confirmed = messageMetadata.IsEligible()
				}

				addToHistory(message, confirmed)

				visualizerWorkerPool.TrySubmit(message, confirmed)
			})
		})
	})

	notifyNewTip := events.NewClosure(func(tipEvent *tangle.TipEvent) {
		// TODO: handle weak tips
		if tipEvent.TipType == tangle.StrongTip {
			visualizerWorkerPool.TrySubmit(tipEvent.TipType, tipEvent.MessageID, true)
		}
	})

	notifyDeletedTip := events.NewClosure(func(tipEvent *tangle.TipEvent) {
		// TODO: handle weak tips
		if tipEvent.TipType == tangle.StrongTip {
			visualizerWorkerPool.TrySubmit(tipEvent.TipType, tipEvent.MessageID, false)
		}
	})

	if err := daemon.BackgroundWorker("Dashboard[Visualizer]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle().Storage.Events.MessageStored.Attach(notifyNewMsg)
		defer messagelayer.Tangle().Storage.Events.MessageStored.Detach(notifyNewMsg)
		messagelayer.Tangle().ConsensusManager.Events.MessageOpinionFormed.Attach(notifyNewMsg)
		defer messagelayer.Tangle().ConsensusManager.Events.MessageOpinionFormed.Detach(notifyNewMsg)
		messagelayer.Tangle().TipManager.Events.TipAdded.Attach(notifyNewTip)
		defer messagelayer.Tangle().TipManager.Events.TipAdded.Detach(notifyNewTip)
		messagelayer.Tangle().TipManager.Events.TipRemoved.Attach(notifyDeletedTip)
		defer messagelayer.Tangle().TipManager.Events.TipRemoved.Detach(notifyDeletedTip)
		visualizerWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[Visualizer] ...")
		visualizerWorkerPool.Stop()
		log.Info("Stopping Dashboard[Visualizer] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func setupVisualizerRoutes(routeGroup *echo.Group) {
	routeGroup.GET("/visualizer/history", func(c echo.Context) (err error) {
		msgHistoryMutex.RLock()
		defer msgHistoryMutex.RUnlock()

		cpyHistory := make([]*tangle.Message, len(msgHistory))
		copy(cpyHistory, msgHistory)

		var res []vertex
		for _, msg := range cpyHistory {
			res = append(res, vertex{
				ID:              msg.ID().Base58(),
				StrongParentIDs: msg.StrongParents().ToStrings(),
				WeakParentIDs:   msg.WeakParents().ToStrings(),
				IsOpinionFormed: msgOpinionFormed[msg.ID().Base58()],
				IsTx:            msg.Payload().Type() == ledgerstate.TransactionType,
			})
		}

		return c.JSON(http.StatusOK, history{Vertices: res})
	})
}

func addToHistory(msg *tangle.Message, opinionFormed bool) {
	msgHistoryMutex.Lock()
	defer msgHistoryMutex.Unlock()
	if _, exist := msgOpinionFormed[msg.ID().Base58()]; exist {
		msgOpinionFormed[msg.ID().Base58()] = opinionFormed
		return
	}

	// remove 100 old msgs if the slice is full
	if len(msgHistory) >= maxMsgHistorySize {
		for i := 0; i < numHistoryToRemove; i++ {
			delete(msgOpinionFormed, msgHistory[i].ID().Base58())
		}
		msgHistory = append(msgHistory[:0], msgHistory[numHistoryToRemove:maxMsgHistorySize]...)
	}
	// add new msg
	msgHistory = append(msgHistory, msg)
	msgOpinionFormed[msg.ID().Base58()] = opinionFormed
}
