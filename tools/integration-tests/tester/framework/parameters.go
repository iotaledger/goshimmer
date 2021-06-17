package framework

import (
	"fmt"
	"time"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/tools/integration-tests/tester/framework/config"
)

const (
	// ports
	apiPort     = 8080
	gossipPort  = 14666
	peeringPort = 14626
	fpcPort     = 10895

	containerNameTester      = "/tester"
	containerNameEntryNode   = "entry_node"
	containerNameReplica     = "replica_"
	containerNameDrand       = "drand_"
	containerNameSuffixPumba = "_pumba"

	logsDir             = "/tmp/logs/"
	dockerLogsPrefixLen = 8
)

var (
	// GenesisTokenAmount is the amount of tokens in the genesis output.
	GenesisTokenAmount = 1000000000000000
	GenesisSeed        = []byte{
		95, 76, 224, 164, 168, 80, 141, 174, 133, 77, 153, 100, 4, 202, 113,
		104, 71, 130, 88, 200, 46, 56, 243, 121, 216, 236, 70, 146, 234, 158, 206, 230,
	}

	// MasterSeed denotes the identity seed of the master peer.
	MasterSeed = []byte{
		37, 202, 104, 245, 5, 80, 107, 111, 131, 48, 156, 82, 158, 253, 215, 219,
		229, 168, 205, 88, 39, 177, 106, 25, 78, 47, 62, 28, 242, 12, 6, 237,
	}
)

// CreateNetworkConfig is the config for optional plugins passed through NewNetwork.
type CreateNetworkConfig struct {
	// StartSynced specifies whether all node in the network start synced.
	StartSynced bool
	// Autopeering specifies whether autopeering or manual peering is used.
	Autopeering bool
	// Faucet specifies whether the first peer should have the faucet enabled.
	Faucet bool // TODO: which tests require a faucet?
	// FPC specified whether FPC is enabled.
	FPC bool // TODO: which tests require FPC?
}

// PeerConfig specifies the default config of a standard GoShimmer peer.
var PeerConfig = config.GoShimmer{
	DisabledPlugins: []string{"portcheck", "dashboard", "analysis-client", "profiling", "clock"},
	POW: config.POW{
		Enabled:    true,
		Difficulty: 2,
	},
	Webapi: config.Webapi{
		Enabled:     true,
		BindAddress: fmt.Sprintf(":%d", apiPort),
	},
	Autopeering: config.Autopeering{
		Enabled: false,
		Port:    peeringPort,
	},
	MessageLayer: config.MessageLayer{
		Enabled: true,
		FCOB: struct{ QuarantineTime int }{
			QuarantineTime: 2,
		},
		Snapshot: struct {
			File        string
			GenesisNode string
		}{
			File:        "/assets/7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih.bin",
			GenesisNode: "", // TODO: what is this?
		},
		TangleTimeWindow: 30 * time.Second,
		StartSynced:      false,
	},
	FPC: config.FPC{
		Enabled:                 false,
		BindAddress:             fmt.Sprintf(":%d", fpcPort),
		RoundInterval:           5,
		TotalRoundsFinalization: 10,
	},
	Faucet: config.Faucet{
		Enabled:               false,
		Seed:                  base58.Encode(GenesisSeed),
		TokensPerRequest:      1000000,
		PowDifficulty:         3,
		PreparedOutputsCounts: 10,
	},
	Mana: config.Mana{
		Enabled:                       true,
		AllowedAccessFilterEnabled:    false,
		AllowedConsensusFilterEnabled: false,
	},
	DRNG: config.DRNG{
		Enabled: false,
	},
	Activity: config.Activity{
		Enabled:              false,
		BroadcastIntervalSec: 1,
	},
}

// EntryNodeConfig specifies the default config of a standard GoShimmer entry node.
var EntryNodeConfig = config.GoShimmer{
	DisabledPlugins: append(PeerConfig.DisabledPlugins,
		"gossip", "issuer", "metrics", "valuetransfers", "consensus"),
	POW:    PeerConfig.POW,
	Webapi: PeerConfig.Webapi,
	Autopeering: config.Autopeering{
		Enabled:    true,
		Port:       peeringPort,
		EntryNodes: nil,
	},
	MessageLayer: config.MessageLayer{
		Enabled: false,
	},
	FPC: config.FPC{
		Enabled: false,
	},
	Faucet: config.Faucet{
		Enabled: false,
	},
	Mana: config.Mana{
		Enabled: false,
	},
	DRNG: config.DRNG{
		Enabled: false,
	},
	Activity: config.Activity{
		Enabled: false,
	},
}
