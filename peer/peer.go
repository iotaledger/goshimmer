package peer

import (
	"net"
	"time"

	"github.com/wollac/autopeering/identity"
	"github.com/wollac/autopeering/salt"
)

type Peer struct {
	Identity    *identity.Identity
	Address     net.IP
	PeeringPort uint16
	GossipPort  uint16
	Salt        *salt.Salt
	LastSeen    time.Time
}
