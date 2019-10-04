package discover

import (
	"github.com/wollac/autopeering/peer"
	"go.uber.org/zap"
)

// Config holds discovery related settings.
type Config struct {
	// These settings are required and configure the listener:
	Log *zap.SugaredLogger

	// These settings are optional:
	Bootnodes []*peer.Peer // list of bootstrap nodes
}
