package discover

import (
	"github.com/iotaledger/autopeering-sim/peer"
	"go.uber.org/zap"
)

// Config holds discovery related settings.
type Config struct {
	// These settings are required and configure the listener:
	Log *zap.SugaredLogger

	// These settings are optional:
	MasterPeers []*peer.Peer // list of bootstrap nodes
}
