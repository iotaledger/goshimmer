package portcheck

import (
	"net"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/banner"
)

// PluginName is the name of the port check plugin.
const PluginName = "PortCheck"

var (
	// Plugin is the plugin instance of the port check plugin.
	Plugin *node.Plugin
	log    *logger.Logger

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Local     *peer.Local
	Discovery *discover.Protocol `optional:"true"`
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(*node.Plugin) {
	if deps.Discovery == nil {
		log.Infof("Skipping port check since autopeering plugin is disabled")
		return
	}
	log.Info("Testing autopeering service ...")
	checkAutopeeringConnection()
	log.Info("Testing autopeering service ... done")
}

// check that discovery is working and the port is open
func checkAutopeeringConnection() {
	peering := deps.Local.Services().Get(service.PeeringKey)

	// resolve the bind address
	localAddr, err := net.ResolveUDPAddr(peering.Network(), autopeering.Parameters.BindAddress)
	if err != nil {
		log.Fatalf("Error resolving %s: %v", autopeering.Parameters.BindAddress, err)
	}
	// open a connection
	conn, err := net.ListenUDP(peering.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer conn.Close()

	// create a new discovery server for the port check
	disc := discover.New(deps.Local, discovery.ProtocolVersion, discovery.Parameters.NetworkVersion, discover.Logger(log))
	srv := server.Serve(deps.Local, conn, log, disc)
	defer srv.Close()

	disc.Start(srv)
	defer disc.Close()

	for _, entryNode := range deps.Discovery.GetMasterPeers() {
		err = disc.Ping(entryNode)
		if err == nil {
			log.Infof("Pong received from %s", entryNode.IP())
			break
		}
		log.Warnf("Error pinging entry node %s: %s", entryNode.IP(), err)
	}

	if err != nil {
		log.Fatalf("Please check that %s is publicly reachable at port %d/%s",
			banner.AppName, peering.Port(), peering.Network())
	}
}
