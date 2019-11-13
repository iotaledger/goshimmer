package selection

import (
	"time"

	"github.com/wollac/autopeering/peer"
	"go.uber.org/zap"
)

// Config holds settings for the peer selection.
type Config struct {
	// Logger
	Log *zap.SugaredLogger

	// Lifetime of the local private salt
	SaltLifetime time.Duration

	// Whether all the neighbors are dropped when the distance is updated
	DropNeighbors bool

	// These settings are optional:
	PeeringRequest chan<- PeeringRequest // sent peering requests and their status are written to this channel
	DropRequest    chan<- DropRequest    // sent drop requests are written to this channel
}

// PeeringRequest is the status of a peering request sent by the local peer. They are written to the corresponding channel if configured.
// This is for simulations only, do not use in production.
type PeeringRequest struct {
	Self   peer.ID // ID of the peer sending the request
	ID     peer.ID // ID of the target peer
	Status bool    // status returned by the target peer
}

// DropRequest is a drop request sent by the local peer. They are written to the corresponding channel if configured.
// This is for simulations only, do not use in production.
type DropRequest struct {
	Self peer.ID // ID of the peer sending the request
	ID   peer.ID // ID of the target peer
}
