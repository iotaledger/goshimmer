package portcheck

import (
	"net"

	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	PLUGIN_NAME = "PortCheck"
)

var (
	PLUGIN = node.NewPlugin(PLUGIN_NAME, node.Enabled, configure, run)
	log    *logger.Logger
)

func configure(*node.Plugin) {
	log = logger.NewLogger(PLUGIN_NAME)
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
	localAddr, err := net.ResolveUDPAddr(peering.Network(), autopeering.GetBindAddress())
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CFG_BIND, err)
	}

	// open a connection
	conn, err := net.ListenUDP(peering.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer conn.Close()
	// start a server
	srv := server.Serve(local.GetInstance(), conn, log, autopeering.Discovery)
	defer srv.Close()

	// set a temporary sender without starting the actual discovery
	autopeering.Discovery.Sender = srv
	defer func() { autopeering.Discovery.Sender = nil }()

	for _, master := range autopeering.Discovery.GetMasterPeers() {
		err = autopeering.Discovery.Ping(master)
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
