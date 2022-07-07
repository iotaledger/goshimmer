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
	Plugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](func(event *node.InitEvent) {
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
	deps.Tangle.Booker.Events.BlockBooked.Attach(event.NewClosure(func(event *tangle.BlockBookedEvent) {
		onReceiveBlockFromBlockLayer(event.BlockID)
	}))
	configureWebAPI()
}

func onReceiveBlockFromBlockLayer(blockID tangle.BlockID) {
	var chatEvent *chat.BlockReceivedEvent
	deps.Tangle.Storage.Block(blockID).Consume(func(block *tangle.Block) {
		if block.Payload().Type() != chat.Type {
			return
		}
		chatPayload := block.Payload().(*chat.Payload)
		chatEvent = &chat.BlockReceivedEvent{
			From:      chatPayload.From(),
			To:        chatPayload.To(),
			Block:     chatPayload.Block(),
			Timestamp: block.IssuingTime(),
			BlockID:   block.ID().Base58(),
		}
	})

	if chatEvent == nil {
		return
	}

	deps.Chat.Events.BlockReceived.Trigger(chatEvent)
}
