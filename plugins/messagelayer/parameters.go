package messagelayer

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the messagelayer plugin.
type ParametersDefinition struct {
	// TangleWidth can be used to specify the number of tips the Tangle tries to maintain.
	TangleWidth int `default:"0" usage:"the width of the Tangle"`

	// Snapshot contains snapshots related configuration parameters.
	Snapshot struct {
		// File is the path to the snapshot file.
		File string `default:"./snapshot.bin" usage:"the path to the snapshot file"`
		// GenesisNode is the identity of the node that is allowed to attach to the Genesis message.
		GenesisNode string `default:"Gm7W191NDnqyF7KJycZqK7V6ENLwqxTwoKQN4SmpkB24" usage:"the node (base58 public key) that is allowed to attach to the genesis message"`
	}

	// FCOB contains parameters related to the transaction quarantine time before applying (if necessary) FPC.
	FCOB struct {
		// QuarantineTime determines the duration of the the first half of the quarantime time of the FCoB rule, in seconds.
		QuarantineTime int `default:"2" usage:"the duration for the first half of the quarantine time of the FCoB rule in sec"`
	}

	// TangleTimeWindow defines the time window in which the node considers itself as synced according to TangleTime.
	TangleTimeWindow time.Duration `default:"2m" usage:"the time window in which the node considers itself as synced according to TangleTime"`

	// StartSynced defines if the node should start as synced.
	StartSynced bool `default:"false" usage:"start as synced"`
}

// FPCParametersDefinition contains the definition of parameters used by the FPC consensus.
type FPCParametersDefinition struct {
	// BindAddress defines on which address the FPC service should listen.
	BindAddress string `default:"0.0.0.0:10895" usage:"the bind address on which the FPC vote server binds to"`

	// Listen defines if the FPC service should listen.
	Listen bool `default:"true" usage:"if the FPC service should listen"`

	// RoundInterval defines how long a round lasts (in seconds).
	RoundInterval int64 `default:"10" usage:"FPC round interval [s]"`

	// QuerySampleSize defines how many nodes will be queried each round.
	QuerySampleSize int `default:"21" usage:"Size of the voting quorum (k)"`

	// TotalRoundsFinalization defines the amount of rounds a vote context's opinion needs to stay the same to be considered final. Also called 'l'.
	TotalRoundsFinalization int `default:"10" usage:"The number of rounds opinion needs to stay the same to become final (l)"`

	// DRNGInstanceID the instanceID of the dRNG to be used with FPC.
	DRNGInstanceID uint32 `default:"1339" usage:"The instanceID of the dRNG to be used with FPC"`

	// AwaitOffset defines the max amount of time (in seconds) to wait for the next dRNG round after the excected time has elapsed.
	AwaitOffset int64 `default:"3" usage:"The max amount of time (in seconds) to wait for the next dRNG round after the excected time has elapsed"`

	// DefaultRandomness defines default randomness used by FPC when no random is received from the dRNG.
	DefaultRandomness float64 `default:"0.5" usage:"The default randomness used by FPC when no random is received from the dRNG"`
}

// StatementParametersDefinition contains the definition of the parameters used by the FPC statements in the tangle.
type StatementParametersDefinition struct {
	// WaitForStatement is the time in seconds for which the node wait for receiving the new statement.
	WaitForStatement int `default:"5" usage:"the time in seconds for which the node wait for receiving the new statement"`

	// WriteStatement defines if the node should write statements.
	WriteStatement bool `default:"true" usage:"if the node should make statements"`

	// ReadManaThreshold defines the Mana threshold to accept a statement.
	ReadManaThreshold float64 `default:"1.0" usage:"Value describing the percentage of top mana nodes to accept a statement from"`

	// WriteManaThreshold defines the Mana threshold to write a statement.
	WriteManaThreshold float64 `default:"0.7" usage:"Value describing the percentage of top mana nodes that can write a statement"`

	// CleanInterval defines the time interval [in minutes] for cleaning the statement registry.
	CleanInterval int `default:"5" usage:"the time in minutes after which the node cleans the statement registry"`

	// DeleteAfter defines the time [in minutes] after which older statements are deleted from the registry.
	DeleteAfter int `default:"5" usage:"the time in minutes after which older statements are deleted from the registry"`
}

// ManaParametersDefinition contains the definition of the parameters used by the mana plugin.
type ManaParametersDefinition struct {
	// EmaCoefficient1 defines the coefficient used for Effective Base Mana 1 (moving average) calculation.
	EmaCoefficient1 float64 `default:"0.00003209" usage:"coefficient used for Effective Base Mana 1 (moving average) calculation"`
	// EmaCoefficient2 defines the coefficient used for Effective Base Mana 2 (moving average) calculation.
	EmaCoefficient2 float64 `default:"0.0057762265" usage:"coefficient used for Effective Base Mana 1 (moving average) calculation"`
	// Decay defines the decay coefficient used for Base Mana 2 calculation.
	Decay float64 `default:"0.00003209" usage:"decay coefficient used for Base Mana 2 calculation"`
	// AllowedAccessPledge defines the list of nodes that access mana is allowed to be pledged to.
	AllowedAccessPledge []string `usage:"list of nodes that access mana is allowed to be pledged to"`
	// AllowedAccessFilterEnabled defines if access mana pledge filter is enabled.
	AllowedAccessFilterEnabled bool `default:"false" usage:"list of nodes that consensus mana is allowed to be pledge to"`
	// AllowedConsensusPledge defines the list of nodes that consensus mana is allowed to be pledged to.
	AllowedConsensusPledge []string `usage:"list of nodes that consensus mana is allowed to be pledge to"`
	// AllowedConsensusFilterEnabled defines if consensus mana pledge filter is enabled.
	AllowedConsensusFilterEnabled bool `default:"false" usage:"if filtering on consensus mana pledge nodes is enabled"`
	// EnableResearchVectors determines if research mana vector should be used or not. To use the Mana Research
	// Grafana Dashboard, this should be set to true.
	EnableResearchVectors bool `default:"false" usage:"enable mana research vectors"`
	// PruneConsensusEventLogsInterval defines the interval to check and prune consensus event logs storage.
	PruneConsensusEventLogsInterval time.Duration `default:"5m" usage:"interval to check and prune consensus event storage"`
	// VectorsCleanupInterval defines the interval to clean empty mana nodes from the base mana vectors.
	VectorsCleanupInterval time.Duration `default:"30m" usage:"interval to cleanup empty mana nodes from the mana vectors"`
	// DebuggingEnabled defines if the mana plugin responds to queries while not being in sync or not.
	DebuggingEnabled bool `default:"false" usage:"if mana plugin responds to queries while not in sync"`
	// SnapshotResetTime defines if the aMana Snapshot should be reset to the current Time.
	SnapshotResetTime bool `default:"false" usage:"when loading snapshot reset to current time when true"`
}

// RateSetterParametersDefinition contains the definition of the parameters used by the Rate Setter.
type RateSetterParametersDefinition struct {
	// Initial defines the initial rate of rate setting.
	Initial float64 `default:"100000" usage:"the initial rate of rate setting"`
}

// SchedulerParametersDefinition contains the definition of the parameters used by the Scheduler.
type SchedulerParametersDefinition struct {
	// MaxBufferSize defines the maximum buffer size (in bytes).
	MaxBufferSize int `default:"100000000" usage:"maximum buffer size (in bytes)"` // 100 MB
	// SchedulerRate defines the frequency to schedule a message.
	Rate string `default:"5ms" usage:"message scheduling interval [time duration string]"`
}

// Parameters contains the general configuration used by the messagelayer plugin.
var Parameters = &ParametersDefinition{}

// FPCParameters contains the FPC configuration used by the messagelayer plugin.
var FPCParameters = &FPCParametersDefinition{}

// StatementParameters contains the FPC statement configuration used by the messagelayer plugin.
var StatementParameters = &StatementParametersDefinition{}

// ManaParameters contains the mana configuration used by the messagelayer plugin.
var ManaParameters = &ManaParametersDefinition{}

// RateSetterParameters contains the rate setter configuration used by the messagelayer plugin.
var RateSetterParameters = &RateSetterParametersDefinition{}

// SchedulerParameters contains the scheduler configuration used by the messagelayer plugin.
var SchedulerParameters = &SchedulerParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "messageLayer")
	configuration.BindParameters(FPCParameters, "fpc")
	configuration.BindParameters(StatementParameters, "statement")
	configuration.BindParameters(ManaParameters, "mana")
	configuration.BindParameters(RateSetterParameters, "rateSetter")
	configuration.BindParameters(SchedulerParameters, "scheduler")
}
