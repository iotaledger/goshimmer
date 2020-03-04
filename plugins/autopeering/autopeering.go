package autopeering

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/autopeering/transport"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/gossip"
)

var (
	// Discovery is the peer discovery protocol.
	Discovery *discover.Protocol
	// Selection is the peer selection protocol.
	Selection *selection.Protocol

	ErrParsingMasterNode = errors.New("can't parse master node")

	log *logger.Logger
)

func configureAP() {
	masterPeers, err := parseEntryNodes()
	if err != nil {
		log.Errorf("Invalid entry nodes; ignoring: %v", err)
	}
	log.Debugf("Master peers: %v", masterPeers)

	Discovery = discover.New(local.GetInstance(),
		discover.Logger(log.Named("disc")),
		discover.Version(config.Node.GetUint32(CFG_VERSION)),
		discover.MasterPeers(masterPeers),
	)

	log.Infof("Protocol Version: %v", discover.VersionNum)
	// enable peer selection only when gossip is enabled
	if !node.IsSkipped(gossip.PLUGIN) {
		Selection = selection.New(local.GetInstance(), Discovery,
			selection.Logger(log.Named("sel")),
			selection.NeighborValidator(selection.ValidatorFunc(isValidNeighbor)),
		)
	}
}

// isValidNeighbor checks whether a peer is a valid neighbor.
func isValidNeighbor(p *peer.Peer) bool {
	// gossip must be supported
	gossipAddr := p.Services().Get(service.GossipKey)
	if gossipAddr == nil {
		return false
	}
	// the host for the gossip and peering service must be identical
	gossipHost, _, err := net.SplitHostPort(gossipAddr.String())
	if err != nil {
		return false
	}
	peeringAddr := p.Services().Get(service.PeeringKey)
	peeringHost, _, err := net.SplitHostPort(peeringAddr.String())
	if err != nil {
		return false
	}
	return gossipHost == peeringHost
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping " + name + " ... done")

	lPeer := local.GetInstance()
	// use the port of the peering service
	peeringAddr := lPeer.Services().Get(service.PeeringKey)
	_, peeringPort, err := net.SplitHostPort(peeringAddr.String())
	if err != nil {
		panic(err)
	}
	// resolve the bind address
	address := net.JoinHostPort(config.Node.GetString(local.CFG_BIND), peeringPort)
	localAddr, err := net.ResolveUDPAddr(peeringAddr.Network(), address)
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CFG_BIND, err)
	}

	conn, err := net.ListenUDP(peeringAddr.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer conn.Close()

	// use the UDP connection for transport
	trans := transport.Conn(conn, func(network, address string) (net.Addr, error) { return net.ResolveUDPAddr(network, address) })
	defer trans.Close()

	handlers := []server.Handler{Discovery}
	if Selection != nil {
		handlers = append(handlers, Selection)
	}

	// start a server doing discovery and peering
	srv := server.Serve(lPeer, trans, log.Named("srv"), handlers...)
	defer srv.Close()

	// start the discovery on that connection
	Discovery.Start(srv)
	defer Discovery.Close()

	if Selection != nil {
		// start the peering on that connection
		Selection.Start(srv)
		defer Selection.Close()
	}

	log.Infof("%s started: ID=%s Address=%s/%s", name, lPeer.ID(), peeringAddr.String(), peeringAddr.Network())

	<-shutdownSignal

	log.Infof("Stopping %s ...", name)

	count := lPeer.Database().PersistSeeds()
	log.Infof("%d peers persisted as seeds", count)
}

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range config.Node.GetStringSlice(CFG_ENTRY_NODES) {
		if entryNodeDefinition == "" {
			continue
		}

		parts := strings.Split(entryNodeDefinition, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: master node parts must be 2, is %d", ErrParsingMasterNode, len(parts))
		}
		pubKey, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, fmt.Errorf("%w: can't decode public key: %s", ErrParsingMasterNode, err)
		}

		services := service.New()
		services.Update(service.PeeringKey, "udp", parts[1])

		result = append(result, peer.NewPeer(pubKey, services))
	}

	return result, nil
}
