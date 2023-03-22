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
	// ValidatorActivityWindow is used to define period of inactivity after which validator is removed from the set of active validators.
	ValidatorActivityWindow time.Duration `default:"30s" usage:"define period of inactivity after which validator is removed from the set of active validators"`
	// BootstrapWindow defines the time window in which the node considers itself as synced according to TangleTime.
	BootstrapWindow time.Duration `default:"20s" usage:"the time window in which the node considers itself as bootstrapped according to AcceptanceTime"`
	// Snapshot contains snapshots related configuration parameters.
	Snapshot struct {
		// Path is the path to the snapshot file.
		Path string `default:"./snapshot.bin" usage:"the path of the snapshot file"`
		// Depth defines how many slot diffs are stored in the snapshot, starting from the full ledgerstate.
		Depth int `default:"5" usage:"defines how many slot diffs are stored in the snapshot, starting from the full ledgerstate"`
	}
	// ForkDetectionMinimumDepth defines the minimum depth a fork has to have to be detected.
	ForkDetectionMinimumDepth int64 `default:"3" usage:"the minimum depth a fork has to have to be detected"`
	// MaxAllowedClockDrift defines the maximum drift our wall clock can have to future blocks being received from the network.
	MaxAllowedClockDrift time.Duration `default:"5s" usage:"the maximum drift our wall clock can have to future blocks being received from the network"`
}

// SchedulerParametersDefinition contains the definition of the parameters used by the Scheduler.
type SchedulerParametersDefinition struct {
	// MaxBufferSize defines the maximum buffer size (in number of blocks).
	MaxBufferSize int `default:"10000" usage:"maximum buffer size (in number of blocks)"` // 300 blocks
	// Rate defines the frequency to schedule a block.
	Rate time.Duration `default:"5ms" usage:"block scheduling interval [time duration string]"` // 200 blocks per second
	// ConfirmedBlockThreshold time threshold after which confirmed blocks are not scheduled [time duration string]
	ConfirmedBlockThreshold time.Duration `default:"1m" usage:"time threshold after which confirmed blocks are not scheduled [time duration string]"`
	// MaxDeficit defines the maximum defict a node can build up.
	MaxDeficit int `default:"10" usage:"max deficit (in units of work)"` // 10 units of work
}

// NotarizationParametersDefinition contains the definition of the parameters used by the notarization plugin.
type NotarizationParametersDefinition struct {
	// MinSlotCommittableAge defines the min age of a committable slot.
	MinSlotCommittableAge int64 `default:"6" usage:"min age of a committable slot denoted in slots"`
}

// DatabaseParametersDefinition contains the definition of configuration parameters used by the storage layer.
type DatabaseParametersDefinition struct {
	// Directory defines the directory of the database.
	Directory string `default:"db" usage:"path to the database directory"`

	// InMemory defines whether to use an in-memory database.
	InMemory bool `default:"false" usage:"whether the database is only kept in memory and not persisted"`

	MaxOpenDBs       int    `default:"10" usage:"maximum number of open database instances"`
	PruningThreshold uint64 `default:"360" usage:"how many confirmed slots should be retained"`
	DBGranularity    int64  `default:"1" usage:"how many slots should be contained in a single DB instance"`

	// ForceCacheTime is a new global cache time in seconds for object storage.
	ForceCacheTime time.Duration `default:"-1s" usage:"interval of time for which objects should remain in memory. Zero time means no caching, negative value means use defaults"`
	Settings       struct {
		// Path is the path to the settings file.
		FileName string `default:"settings.bin" usage:"the file name of the settings file, relative to the database directory"`
	}
}

// DebugParametersDefinition contains the definition of configuration parameters used for debugging purposes.
type DebugParametersDefinition struct {
	PanicOnForkDetection bool `default:"false" usage:"whether to panic if a network fork is detected or if the normal chain switching is allowed to happen"`
}

// Parameters contains the general configuration used by the blocklayer plugin.
var Parameters = &ParametersDefinition{}

// SchedulerParameters contains the scheduler configuration used by the blocklayer plugin.
var SchedulerParameters = &SchedulerParametersDefinition{}

// NotarizationParameters contains the configuration used by the notarization plugin.
var NotarizationParameters = &NotarizationParametersDefinition{}

// DatabaseParameters contains configuration parameters used by Database.
var DatabaseParameters = &DatabaseParametersDefinition{}

// DebugParameters contains the configuration parameters used for debugging purposes.
var DebugParameters = &DebugParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "protocol")
	config.BindParameters(SchedulerParameters, "scheduler")
	config.BindParameters(NotarizationParameters, "notarization")
	config.BindParameters(DatabaseParameters, "database")
	config.BindParameters(DebugParameters, "debug")
}
