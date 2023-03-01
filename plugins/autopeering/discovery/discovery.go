package discovery

import (
	"net"
	"strings"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/logger"
)

// autopeering constants.
const (
	ProtocolVersion = 0 // update on protocol changes

	entryNodeParts = 2
)

// ErrParsingEntryNode is returned for an invalid entry node.
var ErrParsingEntryNode = errors.New("cannot parse entry node")

// CreatePeerDisc creates a discover protocol instance.
func CreatePeerDisc(localID *peer.Local) *discover.Protocol {
	log := logger.NewLogger("Autopeering").Named("disc")

	entryNodes, err := parseEntryNodes()
	if err != nil {
		log.Errorf("Invalid entry nodes; ignoring: %v", err)
	}
	log.Debugf("Entry nodes: %v", entryNodes)

	return discover.New(localID, ProtocolVersion, Parameters.NetworkVersion,
		discover.Logger(log),
		discover.MasterPeers(entryNodes),
	)
}

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range Parameters.EntryNodes {
		if entryNodeDefinition == "" {
			continue
		}

		parts := strings.Split(entryNodeDefinition, "@")
		if len(parts) != entryNodeParts {
			return nil, errors.WithMessagef(ErrParsingEntryNode, "entry node information must contains %d parts, is %d", entryNodeParts, len(parts))
		}
		pubKey, err := base58.Decode(parts[0])
		if err != nil {
			return nil, errors.WithMessagef(ErrParsingEntryNode, "invalid public key: %s", err.Error())
		}
		addr, err := net.ResolveUDPAddr("udp", parts[1])
		if err != nil {
			return nil, errors.WithMessagef(ErrParsingEntryNode, "host cannot be resolved: %s", err.Error())
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
