package graph

import (
	"github.com/iotaledger/goshimmer/plugins/banner"
	flag "github.com/spf13/pflag"
)

const (
	CFG_WEBROOT      = "graph.webrootPath"
	CFG_SOCKET_IO    = "graph.socketioPath"
	CFG_DOMAIN       = "graph.domain"
	CFG_BIND_ADDRESS = "graph.bindAddress"
	CFG_NETWORK      = "graph.networkName"
)

func init() {
	flag.String(CFG_WEBROOT, "IOTAtangle/webroot", "Path to IOTA Tangle Visualiser webroot files")
	flag.String(CFG_SOCKET_IO, "socket.io-client/dist/socket.io.js", "Path to socket.io.js")
	flag.String(CFG_DOMAIN, "", "Set the domain on which IOTA Tangle Visualiser is served")
	flag.String(CFG_BIND_ADDRESS, "127.0.0.1:8082", "the bind address for the IOTA Tangle Visualizer")
	flag.String(CFG_NETWORK, banner.AppName, "Name of the network shown in IOTA Tangle Visualiser")
}
