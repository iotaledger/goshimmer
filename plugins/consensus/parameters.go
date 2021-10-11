package consensus

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// FPCParametersDefinition contains the definition of parameters used by the FPC consensus.
type FPCParametersDefinition struct {
	// BindAddress defines on which address the FPC service should listen.
	BindAddress string `default:"0.0.0.0:10895" usage:"the bind address on which the FPC vote server binds to"`

	// Listen defines if the FPC service should listen.
	Listen bool `default:"true" usage:"if the FPC service should listen"`

	// RoundInterval defines how long a round lasts.
	RoundInterval time.Duration `default:"10s" usage:"FPC round interval"`

	// QuerySampleSize defines how many nodes will be queried each round.
	QuerySampleSize int `default:"21" usage:"Size of the voting quorum (k)"`

	// TotalRoundsFinalization defines the amount of rounds a vote context's opinion needs to stay the same to be considered final. Also called 'l'.
	TotalRoundsFinalization int `default:"10" usage:"The number of rounds opinion needs to stay the same to become final (l)"`

	// DRNGInstanceID the instanceID of the dRNG to be used with FPC.
	DRNGInstanceID uint32 `default:"1339" usage:"The instanceID of the dRNG to be used with FPC"`

	// AwaitOffset defines the max amount of time to wait for the next dRNG round after the expected time has elapsed.
	AwaitOffset time.Duration `default:"3s" usage:"The max amount of time to wait for the next dRNG round after the expected time has elapsed"`

	// DefaultRandomness defines default randomness used by FPC when no random is received from the dRNG.
	DefaultRandomness float64 `default:"0.5" usage:"The default randomness used by FPC when no random is received from the dRNG"`
}

// StatementParametersDefinition contains the definition of the parameters used by the FPC statements in the tangle.
type StatementParametersDefinition struct {
	// WaitForStatement is the time in seconds for which the node wait for receiving the new statement.
	WaitForStatement time.Duration `default:"5s" usage:"the time for which the node wait for receiving the new statement"`

	// WriteStatement defines if the node should write statements.
	WriteStatement bool `default:"true" usage:"if the node should make statements"`

	// ReadManaThreshold defines the Mana threshold to accept a statement.
	ReadManaThreshold float64 `default:"1.0" usage:"Value describing the percentage of top mana nodes to accept a statement from"`

	// WriteManaThreshold defines the Mana threshold to write a statement.
	WriteManaThreshold float64 `default:"0.7" usage:"Value describing the percentage of top mana nodes that can write a statement"`

	// CleanInterval defines the time interval [in minutes] for cleaning the statement registry.
	CleanInterval time.Duration `default:"5m" usage:"the time after which the node cleans the statement registry"`

	// DeleteAfter defines the time [in minutes] after which older statements are deleted from the registry.
	DeleteAfter time.Duration `default:"5m" usage:"the time after which older statements are deleted from the registry"`
}

// FPCParameters contains the FPC configuration used by the consensus plugin.
var FPCParameters = &FPCParametersDefinition{}

// StatementParameters contains the FPC statement configuration used by the consensus plugin.
var StatementParameters = &StatementParametersDefinition{}

func init() {
	configuration.BindParameters(FPCParameters, "fpc")
	configuration.BindParameters(StatementParameters, "statement")
}
