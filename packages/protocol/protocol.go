package protocol

import (
	"path/filepath"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/database"
)

// region Protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type Protocol struct {
	network  *network.Network
	settings *Settings
	// dispatcher      *dispatcher.Dispatcher
	// solidification  *solidification.Solidification

	optsNodeID           string
	optsSettingsFile     string
	optsDBManagerOptions []options.Option[database.Manager]
	// optsSolidificationOptions []options.Option[solidification.Solidification]

	*logger.Logger
}

func New(networkInstance *network.Network, log *logger.Logger, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Logger: log,

		optsNodeID:       "iota",
		optsSettingsFile: "settings.bin",
	}, opts, func(n *Protocol) {
		n.network = networkInstance
		n.settings = NewSettings(filepath.Join(n.optsNodeID, n.optsSettingsFile))

		// n.databaseManager = database.NewManager(n.optsDBManagerOptions...)

		// network.OnReceivePacket(protocol.ProcessPacket)

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

func WithDBManagerOptions(opts ...options.Option[database.Manager]) options.Option[Protocol] {
	return func(n *Protocol) {
		n.optsDBManagerOptions = opts
	}
}

// func WithSolidificationOptions(opts ...options.Option[solidification.Solidification]) options.Option[Protocol] {
// 	return func(n *Protocol) {
// 		n.optsSolidificationOptions = opts
// 	}
// }

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
