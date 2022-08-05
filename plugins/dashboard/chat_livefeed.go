package dashboard

import (
	"context"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/app/chat"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"
)

var (
	chatLiveFeedWorkerCount     = 1
	chatLiveFeedWorkerQueueSize = 50
	chatLiveFeedWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
)

type chatBlk struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Block     string `json:"block"`
	BlockID   string `json:"blockID"`
	Timestamp string `json:"timestamp"`
}

func configureChatLiveFeed() {
	chatLiveFeedWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		newBlock := task.Param(0).(*chat.BlockReceivedEvent)

		broadcastWsBlock(&wsblk{MsgTypeChat, &chatBlk{
			From:      newBlock.From,
			To:        newBlock.To,
			Block:     newBlock.Block,
			BlockID:   newBlock.BlockID,
			Timestamp: newBlock.Timestamp.Format("2 Jan 2006 15:04:05"),
		}})

		task.Return(nil)
	}, workerpool.WorkerCount(chatLiveFeedWorkerCount), workerpool.QueueSize(chatLiveFeedWorkerQueueSize))
}

func runChatLiveFeed() {
	if err := daemon.BackgroundWorker("Dashboard[ChatUpdater]", func(ctx context.Context) {
		notifyNewBlocks := event.NewClosure(func(event *chat.BlockReceivedEvent) {
			chatLiveFeedWorkerPool.TrySubmit(event)
		})
		deps.Chat.Events.BlockReceived.Attach(notifyNewBlocks)

		defer chatLiveFeedWorkerPool.Stop()

		<-ctx.Done()
		log.Info("Stopping Dashboard[ChatUpdater] ...")
		deps.Chat.Events.BlockReceived.Detach(notifyNewBlocks)
		log.Info("Stopping Dashboard[ChatUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
