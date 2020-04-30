package portcheck

import (
	"net"

	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the port check plugin.
const PluginName = "PortCheck"

var (
	// Plugin is the plugin instance of the port check plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	log    *logger.Logger
)

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(*node.Plugin) {
	log.Info("Testing autopeering service ...")
	checkAutopeeringConnection()
	log.Info("Testing autopeering service ... done")
}

// check that discovery is working and the port is open
func checkAutopeeringConnection() {
	peering := local.GetInstance().Services().Get(service.PeeringKey)

	// resolve the bind address
	localAddr, err := net.ResolveUDPAddr(peering.Network(), autopeering.BindAddress())
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CfgBind, err)
	}
	// open a connection
	conn, err := net.ListenUDP(peering.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer conn.Close()

	// create a new discovery server for the port check
	disc := discover.New(local.GetInstance(), autopeering.ProtocolVersion, autopeering.NetworkID, discover.Logger(log))
	srv := server.Serve(local.GetInstance(), conn, log, disc)
	defer srv.Close()

	disc.Start(srv)
	defer disc.Close()

	for _, master := range autopeering.Discovery().GetMasterPeers() {
		err = disc.Ping(master)
		if err == nil {
			log.Infof("Pong received from %s", master.IP())
			break
		}
		log.Warnf("Error pinging entry node %s: %s", master.IP(), err)
	}

	if err != nil {
		log.Fatalf("Please check that %s is publicly reachable at port %d/%s",
			banner.AppName, peering.Port(), peering.Network())
	}
}
