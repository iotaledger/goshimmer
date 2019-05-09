package gossip

import (
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/packages/network"
    "net"
)

const (
    MARSHALLED_NEIGHBOR_TOTAL_SIZE = 1
)

type Neighbor struct {
    Conn     *network.ManagedConnection
    Identity identity.Identity
    Address  net.IP
    Port     uint16
}

func UnmarshalNeighbor(data []byte) (*Neighbor, error) {
    return &Neighbor{}, nil
}

func (neighbor *Neighbor) Marshal() []byte {
    return nil
}

func AddNeighbor() {

}

func GetNeighbor() {

}
