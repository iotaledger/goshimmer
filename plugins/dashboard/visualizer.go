package dashboard

import (
	"net/http"
	"sync"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/labstack/echo"
)

var (
	visualizerWorkerCount     = 1
	visualizerWorkerQueueSize = 500
	visualizerWorkerPool      *workerpool.WorkerPool

	msgHistoryMutex   sync.RWMutex
	msgSolid          map[string]bool
	msgHistory        []*tangle.Message
	maxMsgHistorySize = 1000
)

// vertex defines a vertex in a DAG.
type vertex struct {
	ID              string   `json:"id"`
	StrongParentIDs []string `json:"strongParentIDs"`
	WeakParentIDs   []string `json:"weakParentIDs"`
	IsSolid         bool     `json:"is_solid"`
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
			sendVertex(x, task.Param(1).(*tangle.MessageMetadata))
		case tangle.MessageID:
			sendTipInfo(x, task.Param(1).(bool))
		}

		task.Return(nil)
	}, workerpool.WorkerCount(visualizerWorkerCount), workerpool.QueueSize(visualizerWorkerQueueSize))

	// configure msgHistory, msgSolid
	msgSolid = make(map[string]bool, maxMsgHistorySize)
	msgHistory = make([]*tangle.Message, 0, maxMsgHistorySize)
}

func sendVertex(msg *tangle.Message, messageMetadata *tangle.MessageMetadata) {
	broadcastWsMessage(&wsmsg{MsgTypeVertex, &vertex{
		ID:              msg.ID().String(),
		StrongParentIDs: msg.StrongParents().ToStrings(),
		WeakParentIDs:   msg.WeakParents().ToStrings(),
		IsSolid:         messageMetadata.IsSolid(),
	}}, true)
}

func sendTipInfo(messageID tangle.MessageID, isTip bool) {
	broadcastWsMessage(&wsmsg{MsgTypeTipInfo, &tipinfo{
		ID:    messageID.String(),
		IsTip: isTip,
	}}, true)
}

func runVisualizer() {
	notifyNewMsg := events.NewClosure(func(messageID tangle.MessageID) {
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
			messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
				addToHistory(message, messageMetadata.IsSolid())

				visualizerWorkerPool.TrySubmit(message, messageMetadata)
			})
		})
	})

	notifyNewTip := events.NewClosure(func(messageId tangle.MessageID) {
		visualizerWorkerPool.TrySubmit(messageId, true)
	})

	notifyDeletedTip := events.NewClosure(func(messageId tangle.MessageID) {
		visualizerWorkerPool.TrySubmit(messageId, false)
	})

	if err := daemon.BackgroundWorker("Dashboard[Visualizer]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle().Storage.Events.MessageStored.Attach(notifyNewMsg)
		defer messagelayer.Tangle().Storage.Events.MessageStored.Detach(notifyNewMsg)
		messagelayer.Tangle().Solidifier.Events.MessageSolid.Attach(notifyNewMsg)
		defer messagelayer.Tangle().Solidifier.Events.MessageSolid.Detach(notifyNewMsg)
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
				ID:              msg.ID().String(),
				StrongParentIDs: msg.StrongParents().ToStrings(),
				WeakParentIDs:   msg.WeakParents().ToStrings(),
				IsSolid:         msgSolid[msg.ID().String()],
			})
		}

		return c.JSON(http.StatusOK, history{Vertices: res})
	})
}

func addToHistory(msg *tangle.Message, solid bool) {
	msgHistoryMutex.Lock()
	defer msgHistoryMutex.Unlock()
	if _, exist := msgSolid[msg.ID().String()]; exist {
		msgSolid[msg.ID().String()] = solid
		return
	}

	// remove 100 old msgs if the slice is full
	if len(msgHistory) >= maxMsgHistorySize {
		for i := 0; i < 100; i++ {
			delete(msgSolid, msgHistory[i].ID().String())
		}
		msgHistory = append(msgHistory[:0], msgHistory[100:maxMsgHistorySize]...)
	}
	// add new msg
	msgHistory = append(msgHistory, msg)
	msgSolid[msg.ID().String()] = solid
}
