package waspconn

import (
	"fmt"
	"net"
	"sync"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/waspconn"
	"github.com/iotaledger/goshimmer/packages/waspconn/connector"
	"github.com/iotaledger/goshimmer/packages/waspconn/tangleledger"
	"github.com/iotaledger/goshimmer/packages/waspconn/testing"
	"github.com/iotaledger/goshimmer/packages/waspconn/utxodbledger"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	flag "github.com/spf13/pflag"
)

const (
	PluginName = "WaspConn"

	WaspConnPort          = "waspconn.port"
	WaspConnUtxodbEnabled = "waspconn.utxodbenabled"
)

func init() {
	flag.Int(WaspConnPort, 5000, "port for Wasp connections")
	flag.Bool(WaspConnUtxodbEnabled, false, "is utxodb mocking the value tangle enabled")
}

var (
	plugin  *node.Plugin
	appOnce sync.Once

	log *logger.Logger

	ledger waspconn.Ledger
)

func Plugin() *node.Plugin {
	appOnce.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configPlugin, runPlugin)
	})
	return plugin
}

func configPlugin(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)

	utxodbEnabled := config.Node().Bool(WaspConnUtxodbEnabled)

	if utxodbEnabled {
		ledger = utxodbledger.New()
		testing.Config(plugin, log, ledger)
		log.Infof("configured with UTXODB enabled")
	} else {
		ledger = tangleledger.New()
	}
}

func runPlugin(_ *node.Plugin) {
	log.Debugf("starting WaspConn plugin on port %d", config.Node().Int(WaspConnPort))
	port := config.Node().Int(WaspConnPort)
	err := daemon.BackgroundWorker("WaspConn worker", func(shutdownSignal <-chan struct{}) {
		listenOn := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", listenOn)
		if err != nil {
			log.Errorf("failed to start WaspConn daemon: %v", err)
			return
		}
		defer func() {
			_ = listener.Close()
		}()

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				log.Debugf("accepted connection from %s", conn.RemoteAddr().String())
				go connector.Run(conn, log, ledger, shutdownSignal)
			}
		}()

		log.Debugf("running WaspConn plugin on port %d", port)

		<-shutdownSignal

		log.Infof("Detaching WaspConn from the Value Tangle..")
		go func() {
			ledger.Detach()
			log.Infof("Detaching WaspConn from the Value Tangle..Done")
		}()

	}, shutdown.PriorityTangle) // TODO proper shutdown priority
	if err != nil {
		log.Errorf("failed to start WaspConn daemon: %v", err)
	}
}
