package simnetwork

var (
	Network = make(map[string]*Transport)
)

func NewNetwork(peers []*Transport) {
	for _, peer := range peers {
		Network[peer.localAddr] = peer
	}
}

func Down() {
	for _, t := range Network {
		t.Close()
	}
}
