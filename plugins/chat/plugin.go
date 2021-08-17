package chat

import (
	"sync"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/labstack/echo"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
)

const (
	// PluginName contains the human readable name of the plugin.
	PluginName = "Chat"
)

var (
	// App is the "plugin" instance of the chat application.
	app  *node.Plugin
	once sync.Once

	deps dependencies
)

type dependencies struct {
	dig.In
	Tangle *tangle.Tangle
	Server *echo.Echo
}

// App gets the plugin instance.
func App() *node.Plugin {
	once.Do(func() {
		app = node.NewPlugin(PluginName, node.Enabled, configure)
	})
	return app
}

func configure(_ *node.Plugin) {
	dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	})

	deps.Tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(onReceiveMessageFromMessageLayer))
	configureWebAPI()
}

func onReceiveMessageFromMessageLayer(messageID tangle.MessageID) {
	var chatEvent *ChatEvent
	deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		if message.Payload().Type() != Type {
			return
		}

		chatPayload, _, err := FromBytes(message.Payload().Bytes())
		if err != nil {
			app.LogError(err)
			return
		}

		chatEvent = &ChatEvent{
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

	Events.MessageReceived.Trigger(chatEvent)
}
