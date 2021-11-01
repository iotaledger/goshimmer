package dashboard

import (
	"context"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/iotaledger/goshimmer/packages/chat"
	"github.com/iotaledger/goshimmer/packages/shutdown"
)

var (
	chatLiveFeedWorkerCount     = 1
	chatLiveFeedWorkerQueueSize = 50
	chatLiveFeedWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
)

type chatMsg struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Message   string `json:"message"`
	MessageID string `json:"messageID"`
	Timestamp string `json:"timestamp"`
}

func configureChatLiveFeed() {
	chatLiveFeedWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		newMessage := task.Param(0).(*chat.Event)

		broadcastWsMessage(&wsmsg{MsgTypeChat, &chatMsg{
			From:      newMessage.From,
			To:        newMessage.To,
			Message:   newMessage.Message,
			MessageID: newMessage.MessageID,
			Timestamp: newMessage.Timestamp.Format("2 Jan 2006 15:04:05"),
		}})

		task.Return(nil)
	}, workerpool.WorkerCount(chatLiveFeedWorkerCount), workerpool.QueueSize(chatLiveFeedWorkerQueueSize))
}

func runChatLiveFeed() {
	if err := daemon.BackgroundWorker("Dashboard[ChatUpdater]", func(ctx context.Context) {
		notifyNewMessages := events.NewClosure(func(chatEvent *chat.Event) {
			chatLiveFeedWorkerPool.TrySubmit(chatEvent)
		})
		deps.Chat.Events.MessageReceived.Attach(notifyNewMessages)

		defer chatLiveFeedWorkerPool.Stop()

		<-ctx.Done()
		log.Info("Stopping Dashboard[ChatUpdater] ...")
		deps.Chat.Events.MessageReceived.Detach(notifyNewMessages)
		log.Info("Stopping Dashboard[ChatUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
