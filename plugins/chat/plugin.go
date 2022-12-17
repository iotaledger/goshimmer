package chat

import (
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/app/chat"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
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

	BlockIssuer *blockissuer.BlockIssuer
	Protocol    *protocol.Protocol
	Server      *echo.Echo
	Chat        *chat.Chat
}

func configure(_ *node.Plugin) {
	deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(event.NewClosure(onReceiveBlockFromBlockLayer))
	configureWebAPI()
}

func onReceiveBlockFromBlockLayer(block *booker.Block) {
	if chatPayload, ok := block.Payload().(*chat.Payload); ok {
		deps.Chat.Events.BlockReceived.Trigger(&chat.BlockReceivedEvent{
			From:      chatPayload.From(),
			To:        chatPayload.To(),
			Block:     chatPayload.Block(),
			Timestamp: block.IssuingTime(),
			BlockID:   block.ID().Base58(),
		})
	}
}
