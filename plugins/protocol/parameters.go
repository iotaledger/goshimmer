package protocol

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// ParametersDefinition contains the definition of the parameters used by the blocklayer plugin.
type ParametersDefinition struct {
	// TangleWidth can be used to specify the number of tips the Tangle tries to maintain.
	TangleWidth int `default:"0" usage:"the width of the Tangle"`
	// TimeSinceConfirmationThreshold is used to set the limit for which tips with old unconfirmed blocks in its past cone will not be selected.
	TimeSinceConfirmationThreshold time.Duration `default:"30s" usage:"Time Since Confirmation (TSC) threshold"`
	// Snapshot contains snapshots related configuration parameters.
	Snapshot struct {
		// File is the path to the snapshot file.
		File string `default:"./snapshot.bin" usage:"the path to the snapshot file"`

		// TODO: this should be removed
		// GenesisNode is the identity of the node that is allowed to attach to the Genesis block.
		GenesisNode string `default:"Gm7W191NDnqyF7KJycZqK7V6ENLwqxTwoKQN4SmpkB24" usage:"the node (base58 public key) that is allowed to attach to the genesis block"`
	}

	// BootstrapWindow defines the time window in which the node considers itself as synced according to TangleTime.
	BootstrapWindow time.Duration `default:"20s" usage:"the time window in which the node considers itself as bootstrapped according to AcceptanceTime"`

	// GenesisTime resets the genesis time to the specified value, Unix time in seconds.
	GenesisTime int64 `default:"0" usage:"resets the genesis time to the specified value, unix time in seconds"`
}

// ManaParametersDefinition contains the definition of the parameters used by the mana plugin.
type ManaParametersDefinition struct {
	// Number of epochs past the latest committable epoch for which the base mana vector becomes effective.
	EpochDelay uint `default:"2" usage:"number of epochs past the latest committable epoch for which the base mana vector becomes effective"`
}

// SchedulerParametersDefinition contains the definition of the parameters used by the Scheduler.
type SchedulerParametersDefinition struct {
	// MaxBufferSize defines the maximum buffer size (in number of blocks).
	MaxBufferSize int `default:"300" usage:"maximum buffer size (in number of blocks)"` // 300 blocks
	// Rate defines the frequency to schedule a block.
	Rate time.Duration `default:"34ms" usage:"block scheduling interval [time duration string]"` // 29.4 blocks per second
	// ConfirmedBlockThreshold time threshold after which confirmed blocks are not scheduled [time duration string]
	ConfirmedBlockThreshold time.Duration `default:"1m" usage:"time threshold after which confirmed blocks are not scheduled [time duration string]"`
}

// NotarizationParametersDefinition contains the definition of the parameters used by the notarization plugin.
type NotarizationParametersDefinition struct {
	// MinEpochCommittableAge defines the min age of a committable epoch.
	MinEpochCommittableAge time.Duration `default:"1m" usage:"min age of a committable epoch"`
	// BootstrapWindow when notarization manager is considered to be bootstrapped
	BootstrapWindow time.Duration `default:"2m" usage:"when notarization manager is considered to be bootstrapped"`
	// SnapshotDepth defines how many epoch diffs are stored in the snapshot, starting from the full ledgerstate
	SnapshotDepth int `default:"5" usage:"defines how many epoch diffs are stored in the snapshot, starting from the full ledgerstate"`
}

// Parameters contains the general configuration used by the blocklayer plugin.
var Parameters = &ParametersDefinition{}

// ManaParameters contains the mana configuration used by the blocklayer plugin.
var ManaParameters = &ManaParametersDefinition{}

// SchedulerParameters contains the scheduler configuration used by the blocklayer plugin.
var SchedulerParameters = &SchedulerParametersDefinition{}

// NotarizationParameters contains the configuration used by the notarization plugin.
var NotarizationParameters = &NotarizationParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "protocol")
	config.BindParameters(ManaParameters, "mana")
	config.BindParameters(SchedulerParameters, "scheduler")
	config.BindParameters(NotarizationParameters, "notarization")
}
