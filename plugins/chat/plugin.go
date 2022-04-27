package chat

import (
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/chat"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	// PluginName contains the human-readable name of the plugin.
	PluginName = "Chat"
)

var (
	// Plugin is the "plugin" instance of the chat application.
	Plugin *node.Plugin
	deps   = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure)
	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(chat.NewChat); err != nil {
			Plugin.Panic(err)
		}
	}))
}

type dependencies struct {
	dig.In
	Tangle *tangle.Tangle
	Server *echo.Echo
	Chat   *chat.Chat
}

func configure(_ *node.Plugin) {
	deps.Tangle.Booker.Events.MessageBooked.Attach(event.NewClosure(func(event *tangle.MessageBookedEvent) {
		onReceiveMessageFromMessageLayer(event.MessageID)
	}))
	configureWebAPI()
}

func onReceiveMessageFromMessageLayer(messageID tangle.MessageID) {
	var chatEvent *chat.MessageReceivedEvent
	deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		if message.Payload().Type() != chat.Type {
			return
		}

		chatPayload, _, err := chat.FromBytes(message.Payload().Bytes())
		if err != nil {
			Plugin.LogError(err)
			return
		}

		chatEvent = &chat.MessageReceivedEvent{
			From:      chatPayload.From,
			To:        chatPayload.To,
			Message:   chatPayload.Message,
			Timestamp: message.IssuingTime(),
			MessageID: message.ID().Base58(),
		}
	})

	if chatEvent == nil {
		return
	}

	deps.Chat.Events.MessageReceived.Trigger(chatEvent)
}
