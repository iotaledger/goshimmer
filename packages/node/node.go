package node

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
)

// region Node /////////////////////////////////////////////////////////////////////////////////////////////////////////

type Node struct {
	network         *network.Network
	settings        *Settings
	databaseManager *database.Manager
	// dispatcher      *dispatcher.Dispatcher
	// solidification  *solidification.Solidification

	optsSettingsFile     string
	optsDBManagerOptions []options.Option[database.Manager]
	// optsSolidificationOptions []options.Option[solidification.Solidification]

	*logger.Logger
}

func New(networkInstance *network.Network, log *logger.Logger, opts ...options.Option[Node]) (node *Node) {
	return options.Apply(&Node{
		Logger: log,

		optsSettingsFile: "settings.bin",
	}, opts, func(n *Node) {
		n.network = networkInstance
		n.settings = NewSettings(n.optsSettingsFile)
		n.databaseManager = database.NewManager(n.optsDBManagerOptions...)

		/*
			n.protocol = protocol.New(n.databaseManager, log)
			n.protocol = protocol.New(log)
			n.parser = &dispatcher.Dispatcher{}
			n.solidification = solidification.New(n.protocol, n.network, n.optsSolidificationOptions...)

			n.network.Events.BlockReceived.Attach(network.BlockReceivedHandler(n.protocol.ProcessBlockFromPeer))
		*/
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithDBManagerOptions(opts ...options.Option[database.Manager]) options.Option[Node] {
	return func(n *Node) {
		n.optsDBManagerOptions = opts
	}
}

// func WithSolidificationOptions(opts ...options.Option[solidification.Solidification]) options.Option[Node] {
// 	return func(n *Node) {
// 		n.optsSolidificationOptions = opts
// 	}
// }

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
