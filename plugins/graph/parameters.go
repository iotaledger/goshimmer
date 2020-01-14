package graph

import (
	"github.com/iotaledger/goshimmer/packages/parameter"
)

func init() {

	// "Path to IOTA Tangle Visualiser webroot files"
	parameter.NodeConfig.SetDefault("graph.webrootPath", "IOTAtangle/webroot")

	// "Path to socket.io.js"
	parameter.NodeConfig.SetDefault("graph.socketioPath", "socket.io-client/dist/socket.io.js")

	// "Set the domain on which IOTA Tangle Visualiser is served"
	parameter.NodeConfig.SetDefault("graph.domain", "")

	// "Set the host to which the IOTA Tangle Visualiser listens"
	parameter.NodeConfig.SetDefault("graph.host", "127.0.0.1")

	// "IOTA Tangle Visualiser webserver port"
	parameter.NodeConfig.SetDefault("graph.port", 8083)

	// "Name of the network shown in IOTA Tangle Visualiser"
	parameter.NodeConfig.SetDefault("graph.networkName", "meets HORNET")
}
