package simnetwork

var (
	Network = make(map[string]*Transport)
)

func NewNetwork(peers []*Transport) {
	for _, peer := range peers {
		Network[peer.addr] = peer
	}
}
