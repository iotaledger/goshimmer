package messagelayer

import "github.com/iotaledger/hive.go/configuration"

// Parameters contains the configuration parameters used by the message layer.
var Parameters = struct {
	// TangleWidth can be used to specify the number of tips the Tangle tries to maintain.
	TangleWidth int `default:"0" usage:"the width of the Tangle"`

	// Snapshot contains snapshots related configuration parameters.
	Snapshot struct {
		// File is the path to the snapshot file.
		File string `default:"./snapshot.bin" usage:"the path to the snapshot file"`
	}

	// FCOB contains parameters related to the fast consensus of barcelona.
	FCOB struct {
		AverageNetworkDelay int `default:"5" usage:"the avg. network delay to use for FCoB rules"`
	}
}{}

// FPCParameters contains the configuration parameters used by the FPC consensus.
var FPCParameters = struct {
	// BindAddress defines on which address the FPC service should listen.
	BindAddress string `default:"0.0.0.0:10895" usage:"the bind address on which the FPC vote server binds to"`

	// Listen defines if the FPC service should listen.
	Listen bool `default:"true" usage:"if the FPC service should listen"`

	// RoundInterval defines how long a round lasts (in seconds).
	RoundInterval int64 `default:"10" usage:"FPC round interval [s]"`

	// QuerySampleSize defines how many nodes will be queried each round.
	QuerySampleSize int `default:"21" usage:"Size of the voting quorum (k)"`
}{}

// StatementParameters contains the configuration parameters used by the FPC statements in the tangle.
var StatementParameters = struct {
	// WaitForStatement is the time in seconds for which the node wait for receiving the new statement.
	WaitForStatement int `default:"5" usage:"the time in seconds for which the node wait for receiving the new statement"`

	// WriteStatement defines if the node should write statements.
	WriteStatement bool `default:"false" usage:"if the node should make statements"`

	// ManaThreshold defines the Mana threshold to accept/write a statement.
	ManaThreshold float64 `default:"1" usage:"Mana threshold to accept/write a statement"`

	// CleanInterval defines the time interval [in minutes] for cleaning the statement registry.
	CleanInterval int `default:"5" usage:"the time in minutes after which the node cleans the statement registry"`

	// DeleteAfter defines the time [in minutes] after which older statements are deleted from the registry.
	DeleteAfter int `default:"5" usage:"the time in minutes after which older statements are deleted from the registry"`
}{}

func init() {
	configuration.BindParameters(&Parameters, "messageLayer")
	configuration.BindParameters(&FPCParameters, "fpc")
	configuration.BindParameters(&StatementParameters, "statement")
}
