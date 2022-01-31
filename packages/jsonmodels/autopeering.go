package jsonmodels

// GetNeighborsResponse contains information of the autopeering.
type GetNeighborsResponse struct {
	KnownPeers []Neighbor `json:"known,omitempty"`
	Chosen     []Neighbor `json:"chosen"`
	Accepted   []Neighbor `json:"accepted"`
	Error      string     `json:"error,omitempty"`
}

// Neighbor contains information of a neighbor peer.
type Neighbor struct {
	ID        string        `json:"id"`        // comparable node identifier
	PublicKey string        `json:"publicKey"` // public key used to verify signatures
	Services  []PeerService `json:"services,omitempty"`
}

// PeerService contains information about a neighbor peer service.
type PeerService struct {
	ID      string `json:"id"`      // ID of the service
	Address string `json:"address"` // network address of the service
}
