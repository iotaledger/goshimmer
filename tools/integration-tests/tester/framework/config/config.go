package config

import (
	"time"
)

// GoShimmer defines the config of a GoShimmer node.
type GoShimmer struct {
	// Name specifies the GoShimmer instance.
	Name string
	// DisabledPlugins specifies the plugins that are disabled with a config.
	DisabledPlugins []string
	// Seed specifies identity.
	Seed []byte

	// individual plugin configurations
	POW
	Webapi
	Autopeering
	MessageLayer
	Faucet
	Mana
	Consensus
	FPC
	Activity
	DRNG
}

// POW defines the parameters of the PoW plugin.
type POW struct {
	Enabled bool

	Difficulty int
}

// Webapi defines the parameters of the Web API plugin.
type Webapi struct { // TODO: Why is this not WebAPI?
	Enabled bool

	BindAddress string
}

// Autopeering defines the parameters of the autopeering plugin.
type Autopeering struct { // TODO: Why is this not AutoPeering?
	Enabled bool

	Port       int
	EntryNodes []string
}

// Faucet defines the parameters of the faucet plugin.
type Faucet struct {
	Enabled bool

	Seed                 string
	TokensPerRequest     int
	PowDifficulty        int
	PreparedOutputsCount int
}

// Mana defines the parameters of the Mana plugin.
type Mana struct {
	Enabled bool

	AllowedAccessPledge           []string
	AllowedAccessFilterEnabled    bool
	AllowedConsensusPledge        []string
	AllowedConsensusFilterEnabled bool
}

// MessageLayer defines the parameters used by the message layer.
type MessageLayer struct {
	Enabled bool

	Snapshot struct {
		File        string
		GenesisNode string
	}
	FCOB struct {
		QuarantineTime int
	}

	TangleTimeWindow time.Duration
	StartSynced      bool
}

// Consensus defines the parameters of the consensus plugin.
type Consensus struct {
	Enabled bool
}

// FPC defines the parameters used by the FPC consensus.
type FPC struct {
	Enabled bool

	BindAddress             string
	RoundInterval           int
	TotalRoundsFinalization int
}

// Activity defines the parameters of the activity plugin.
type Activity struct {
	Enabled bool

	BroadcastIntervalSec int
}

// DRNG defines the parameters of the DRNG plugin.
type DRNG struct {
	Enabled bool

	Custom struct {
		InstanceId        int // TODO: should we change that to InstanceID
		Threshold         int
		DistributedPubKey string
		CommitteeMembers  []string
	}
}
