package chat

import (
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/packages/app/chat"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
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
	Tangle *tangleold.Tangle
	Server *echo.Echo
	Chat   *chat.Chat
}

func configure(_ *node.Plugin) {
	deps.Tangle.Booker.Events.BlockBooked.Attach(event.NewClosure(func(event *tangleold.BlockBookedEvent) {
		onReceiveBlockFromBlockLayer(event.BlockID)
	}))
	configureWebAPI()
}

func onReceiveBlockFromBlockLayer(blockID tangleold.BlockID) {
	var chatEvent *chat.BlockReceivedEvent
	deps.Tangle.Storage.Block(blockID).Consume(func(block *tangleold.Block) {
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
