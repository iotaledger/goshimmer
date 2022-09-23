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
	// BootstrapWindow defines the time window in which the node considers itself as synced according to TangleTime.
	BootstrapWindow time.Duration `default:"20s" usage:"the time window in which the node considers itself as bootstrapped according to AcceptanceTime"`
	// GenesisTime resets the genesis time to the specified value, Unix time in seconds.
	GenesisTime int64 `default:"0" usage:"resets the genesis time to the specified value, unix time in seconds"`
	// Snapshot contains snapshots related configuration parameters.
	Snapshot struct {
		// FileName is the path to the snapshot file.
		FileName string `default:"snapshot.bin" usage:"the file name of the snapshot file, relative to the database directory"`
	}
	Settings struct {
		// FileName is the path to the settings file.
		FileName string `default:"settings.bin" usage:"the file name of the settings file, relative to the database directory"`
	}
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

// DatabaseParametersDefinition contains the definition of configuration parameters used by the storage layer.
type DatabaseParametersDefinition struct {
	// Directory defines the directory of the database.
	Directory string `default:"db" usage:"path to the database directory"`

	// InMemory defines whether to use an in-memory database.
	InMemory bool `default:"false" usage:"whether the database is only kept in memory and not persisted"`

	MaxOpenDBs  int   `default:"10" usage:"maximum number of open database instances"`
	Granularity int64 `default:"10" usage:"granularity of the epoch/bucketed database"`

	// ForceCacheTime is a new global cache time in seconds for object storage.
	ForceCacheTime time.Duration `default:"-1s" usage:"interval of time for which objects should remain in memory. Zero time means no caching, negative value means use defaults"`
}

// Parameters contains the general configuration used by the blocklayer plugin.
var Parameters = &ParametersDefinition{}

// ManaParameters contains the mana configuration used by the blocklayer plugin.
var ManaParameters = &ManaParametersDefinition{}

// SchedulerParameters contains the scheduler configuration used by the blocklayer plugin.
var SchedulerParameters = &SchedulerParametersDefinition{}

// NotarizationParameters contains the configuration used by the notarization plugin.
var NotarizationParameters = &NotarizationParametersDefinition{}

// DatabaseParameters contains configuration parameters used by Database.
var DatabaseParameters = &DatabaseParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "protocol")
	config.BindParameters(ManaParameters, "mana")
	config.BindParameters(SchedulerParameters, "scheduler")
	config.BindParameters(NotarizationParameters, "notarization")
	config.BindParameters(DatabaseParameters, "database")
}
