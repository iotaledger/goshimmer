package messagelayer

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the messagelayer plugin.
type ParametersDefinition struct {
	// TangleWidth can be used to specify the number of tips the Tangle tries to maintain.
	TangleWidth int `default:"0" usage:"the width of the Tangle"`
	// TimeSinceConfirmationThreshold is used to set the limit for which tips with old unconfirmed messages in its past cone will not be selected.
	TimeSinceConfirmationThreshold time.Duration `default:"12m" usage:"Time Since Confirmation (TSC) threshold"`
	// Snapshot contains snapshots related configuration parameters.
	Snapshot struct {
		// File is the path to the snapshot file.
		File string `default:"./snapshot.bin" usage:"the path to the snapshot file"`
		// GenesisNode is the identity of the node that is allowed to attach to the Genesis message.
		GenesisNode string `default:"Gm7W191NDnqyF7KJycZqK7V6ENLwqxTwoKQN4SmpkB24" usage:"the node (base58 public key) that is allowed to attach to the genesis message"`
	}

	// TangleTimeWindow defines the time window in which the node considers itself as synced according to TangleTime.
	TangleTimeWindow time.Duration `default:"2m" usage:"the time window in which the node considers itself as synced according to TangleTime"`

	// StartSynced defines if the node should start as synced.
	StartSynced bool `default:"false" usage:"start as synced"`
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
	// MaxBufferSize defines the maximum buffer size (in number of messages).
	MaxBufferSize int `default:"300" usage:"maximum buffer size (in number of messages)"` // 300 messages
	// Rate defines the frequency to schedule a message. `default:"5ms" usage:"message scheduling interval [time duration string]"`
	Rate string `default:"5ms" usage:"message scheduling interval [time duration string]"`
	// ConfirmedMessageThreshold time threshold after which confirmed messages are not scheduled [time duration string]
	ConfirmedMessageThreshold string `default:"1m" usage:"time threshold after which confirmed messages are not scheduled [time duration string]"`
}

// Parameters contains the general configuration used by the messagelayer plugin.
var Parameters = &ParametersDefinition{}

// ManaParameters contains the mana configuration used by the messagelayer plugin.
var ManaParameters = &ManaParametersDefinition{}

// RateSetterParameters contains the rate setter configuration used by the messagelayer plugin.
var RateSetterParameters = &RateSetterParametersDefinition{}

// SchedulerParameters contains the scheduler configuration used by the messagelayer plugin.
var SchedulerParameters = &SchedulerParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "messageLayer")
	configuration.BindParameters(ManaParameters, "mana")
	configuration.BindParameters(RateSetterParameters, "rateSetter")
	configuration.BindParameters(SchedulerParameters, "scheduler")
}
