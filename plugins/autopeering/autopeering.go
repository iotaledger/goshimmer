package autopeering

import (
	"encoding/base64"
	"fmt"
	"net"
	"strings"

	"github.com/iotaledger/goshimmer/packages/autopeering/discover"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/autopeering/selection"
	"github.com/iotaledger/goshimmer/packages/autopeering/server"
	"github.com/iotaledger/goshimmer/packages/autopeering/transport"
	"github.com/iotaledger/goshimmer/packages/parameter"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/pkg/errors"
)

var (
	// Discovery is the peer discovery protocol.
	Discovery *discover.Protocol
	// Selection is the peer selection protocol.
	Selection *selection.Protocol

	log *logger.Logger
)

func configureAP() {
	masterPeers, err := parseEntryNodes()
	if err != nil {
		log.Errorf("Invalid entry nodes; ignoring: %v", err)
	}
	log.Debugf("Master peers: %v", masterPeers)

	Discovery = discover.New(local.GetInstance(), discover.Config{
		Log:         log.Named("disc"),
		MasterPeers: masterPeers,
	})

	// enable peer selection only when gossip is enabled
	if !node.IsSkipped(gossip.PLUGIN) {
		Selection = selection.New(local.GetInstance(), Discovery, selection.Config{
			Log:              log.Named("sel"),
			RequiredServices: []service.Key{service.GossipKey},
		})
	}
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping Auto Peering server ... done")

	addr := local.GetInstance().Services().Get(service.PeeringKey)
	udpAddr, err := net.ResolveUDPAddr(addr.Network(), addr.String())
	if err != nil {
		log.Fatalf("ResolveUDPAddr: %v", err)
	}

	// if the ip is an external ip, set it to unspecified
	if udpAddr.IP.IsGlobalUnicast() {
		if udpAddr.IP.To4() != nil {
			udpAddr.IP = net.IPv4zero
		} else {
			udpAddr.IP = net.IPv6unspecified
		}
	}

	conn, err := net.ListenUDP(addr.Network(), udpAddr)
	if err != nil {
		log.Fatalf("ListenUDP: %v", err)
	}

	// use the UDP connection for transport
	trans := transport.Conn(conn, func(network, address string) (net.Addr, error) { return net.ResolveUDPAddr(network, address) })
	defer trans.Close()

	handlers := []server.Handler{Discovery}
	if Selection != nil {
		handlers = append(handlers, Selection)
	}

	// start a server doing discovery and peering
	srv := server.Listen(local.GetInstance(), trans, log.Named("srv"), handlers...)
	defer srv.Close()

	// start the discovery on that connection
	Discovery.Start(srv)
	defer Discovery.Close()

	if Selection != nil {
		// start the peering on that connection
		Selection.Start(srv)
		defer Selection.Close()
	}

	log.Infof("Auto Peering started: address=%s", srv.LocalAddr())
	log.Debugf("Auto Peering server started: PubKey=%s", base64.StdEncoding.EncodeToString(local.GetInstance().PublicKey()))

	<-shutdownSignal
	log.Info("Stopping Auto Peering server ...")
}

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range parameter.NodeConfig.GetStringSlice(CFG_ENTRY_NODES) {
		if entryNodeDefinition == "" {
			continue
		}

		parts := strings.Split(entryNodeDefinition, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("parseMaster")
		}
		pubKey, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, errors.Wrap(err, "parseMaster")
		}

		services := service.New()
		services.Update(service.PeeringKey, "udp", parts[1])

		result = append(result, peer.NewPeer(pubKey, services))
	}

	return result, nil
}
