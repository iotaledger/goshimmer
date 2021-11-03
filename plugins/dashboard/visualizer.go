package dashboard

import (
	"context"
	"net/http"
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
	"github.com/labstack/echo"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

var (
	visualizerWorkerCount     = 1
	visualizerWorkerQueueSize = 500
	visualizerWorkerPool      *workerpool.NonBlockingQueuedWorkerPool

	msgHistoryMutex    sync.RWMutex
	msgFinalized       map[string]bool
	msgHistory         []*tangle.Message
	maxMsgHistorySize  = 1000
	numHistoryToRemove = 100
)

// vertex defines a vertex in a DAG.
type vertex struct {
	ID              string              `json:"id"`
	ParentIDsByType map[string][]string `json:"parentIDsByType"`
	IsFinalized     bool                `json:"is_finalized"`
	IsTx            bool                `json:"is_tx"`
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
	visualizerWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		switch x := task.Param(0).(type) {
		case *tangle.Message:
			sendVertex(x, task.Param(1).(bool))
		case *tangle.TipEvent:
			sendTipInfo(task.Param(1).(tangle.MessageID), task.Param(2).(bool))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))

	// configure msgHistory, msgSolid
	msgFinalized = make(map[string]bool, maxMsgHistorySize)
	msgHistory = make([]*tangle.Message, 0, maxMsgHistorySize)
}

func sendVertex(msg *tangle.Message, finalized bool) {
	broadcastWsMessage(&wsmsg{MsgTypeVertex, &vertex{
		ID:              msg.ID().Base58(),
		ParentIDsByType: prepareParentReferences(msg),
		IsFinalized:     finalized,
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
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			finalized := deps.Tangle.ConfirmationOracle.IsMessageConfirmed(messageID)
			addToHistory(message, finalized)
			visualizerWorkerPool.TrySubmit(message, finalized)
		})
	})

	notifyNewTip := events.NewClosure(func(tipEvent *tangle.TipEvent) {
		visualizerWorkerPool.TrySubmit(tipEvent, tipEvent.MessageID, true)
	})

	notifyDeletedTip := events.NewClosure(func(tipEvent *tangle.TipEvent) {
		visualizerWorkerPool.TrySubmit(tipEvent, tipEvent.MessageID, false)
	})

	if err := daemon.BackgroundWorker("Dashboard[Visualizer]", func(ctx context.Context) {
		deps.Tangle.Storage.Events.MessageStored.Attach(notifyNewMsg)
		defer deps.Tangle.Storage.Events.MessageStored.Detach(notifyNewMsg)
		deps.Tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(notifyNewMsg)
		defer deps.Tangle.ConfirmationOracle.Events().MessageConfirmed.Detach(notifyNewMsg)
		deps.Tangle.TipManager.Events.TipAdded.Attach(notifyNewTip)
		defer deps.Tangle.TipManager.Events.TipAdded.Detach(notifyNewTip)
		deps.Tangle.TipManager.Events.TipRemoved.Attach(notifyDeletedTip)
		defer deps.Tangle.TipManager.Events.TipRemoved.Detach(notifyDeletedTip)
		<-ctx.Done()
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
				ParentIDsByType: prepareParentReferences(msg),
				IsFinalized:     msgFinalized[msg.ID().Base58()],
				IsTx:            msg.Payload().Type() == ledgerstate.TransactionType,
			})
		}

		return c.JSON(http.StatusOK, history{Vertices: res})
	})
}

func addToHistory(msg *tangle.Message, finalized bool) {
	msgHistoryMutex.Lock()
	defer msgHistoryMutex.Unlock()
	if _, exist := msgFinalized[msg.ID().Base58()]; exist {
		msgFinalized[msg.ID().Base58()] = finalized
		return
	}

	// remove 100 old msgs if the slice is full
	if len(msgHistory) >= maxMsgHistorySize {
		for i := 0; i < numHistoryToRemove; i++ {
			delete(msgFinalized, msgHistory[i].ID().Base58())
		}
		msgHistory = append(msgHistory[:0], msgHistory[numHistoryToRemove:maxMsgHistorySize]...)
	}
	// add new msg
	msgHistory = append(msgHistory, msg)
	msgFinalized[msg.ID().Base58()] = finalized
}
