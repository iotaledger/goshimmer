package discovery

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/mr-tron/base58"
)

// autopeering constants
const (
	ProtocolVersion = 0 // update on protocol changes

	// PluginName is the name of the autopeering plugin.
	PluginName = "Autopeering"
)

var (
	// ErrParsingMasterNode is returned for an invalid master node.
	ErrParsingMasterNode = errors.New("cannot parse master node")

	// the peer discovery protocol
	peerDisc     *discover.Protocol
	peerDiscOnce sync.Once

	networkVersion uint32
)

// Discovery returns the peer discovery instance.
func Discovery() *discover.Protocol {
	peerDiscOnce.Do(createPeerDisc)
	return peerDisc
}

func createPeerDisc() {
	// assure that the logger is available
	log := logger.NewLogger(PluginName).Named("disc")

	networkVersion = uint32(config.Node().Int(CfgNetworkVersion))

	masterPeers, err := parseEntryNodes()
	if err != nil {
		log.Errorf("Invalid entry nodes; ignoring: %v", err)
	}
	log.Debugf("Master peers: %v", masterPeers)

	peerDisc = discover.New(local.GetInstance(), ProtocolVersion, NetworkVersion(),
		discover.Logger(log),
		discover.MasterPeers(masterPeers),
	)
}

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range config.Node().Strings(CfgEntryNodes) {
		if entryNodeDefinition == "" {
			continue
		}

		parts := strings.Split(entryNodeDefinition, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: master node parts must be 2, is %d", ErrParsingMasterNode, len(parts))
		}
		pubKey, err := base58.Decode(parts[0])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid public key: %s", ErrParsingMasterNode, err)
		}
		addr, err := net.ResolveUDPAddr("udp", parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w: host cannot be resolved: %s", ErrParsingMasterNode, err)
		}
		publicKey, _, err := ed25519.PublicKeyFromBytes(pubKey)
		if err != nil {
			return nil, err
		}

		services := service.New()
		services.Update(service.PeeringKey, addr.Network(), addr.Port)

		result = append(result, peer.NewPeer(identity.New(publicKey), addr.IP, services))
	}

	return result, nil
}

// NetworkVersion returns the network version of the autopeering.
func NetworkVersion() uint32 {
	return networkVersion
}
