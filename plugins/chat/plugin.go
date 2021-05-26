package chat

import (
	"sync"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const (
	// PluginName contains the human readable name of the plugin.
	PluginName = "Chat"
)

var (
	// App is the "plugin" instance of the network delay application.
	app  *node.Plugin
	once sync.Once

	// log holds a reference to the logger used by this app.
	log *logger.Logger
)

// App gets the plugin instance.
func App() *node.Plugin {
	once.Do(func() {
		app = node.NewPlugin(PluginName, node.Disabled, configure)
	})
	return app
}

func configure(_ *node.Plugin) {

}

func onReceiveMessageFromMessageLayer(messageID tangle.MessageID) {
	messagelayer.Tangle().Storage.Message(messageID).Consume(func(solidMessage *tangle.Message) {

	})
}
