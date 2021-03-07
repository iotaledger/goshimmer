package messagelayer

import (
	"github.com/iotaledger/goshimmer/packages/configuration"
)

// Parameters contains the configuration parameters used by the message layer.
var Parameters = struct {
	TangleWidth int `default:"0" usage:"the width of the Tangle"`

	Snapshot struct {
		// File is the path to the snapshot file.
		File string `default:"./snapshot.bin" usage:"the path to the snapshot file"`
	}

	FCOB struct {
		AverageNetworkDelay int `default:"5" usage:"the avg. network delay to use for FCoB rules"`
	}
}{}

// FPCParameters contains the configuration parameters used by the FPC consensus.
var FPCParameters = struct {
	BindAddress     string `default:"0.0.0.0:10895" usage:"the bind address on which the FPC vote server binds to"`
	Listen          bool   `default:"true" usage:"if the FPC service should listen"`
	RoundInterval   int64  `default:"10" usage:"FPC round interval [s]"`
	QuerySampleSize int    `default:"21" usage:"Size of the voting quorum (k)"`
}{}

// StatementParameters contains the configuration parameters used by the FPC statements in the tangle.
var StatementParameters = struct {
	WaitForStatement int     `default:"5" usage:"the time in seconds for which the node wait for receiving the new statement"`
	WriteStatement   bool    `default:"false" usage:"if the node should make statements"`
	ManaThreshold    float64 `default:"1" usage:"Mana threshold to accept/write a statement"`
	CleanInterval    int     `default:"5" usage:"the time in minutes after which the node cleans the statement registry"`
	DeleteAfter      int     `default:"5" usage:"the time in minutes after which older statements are deleted from the registry"`
}{}

func init() {
	configuration.DefineParameters(&Parameters, "messageLayer")
	configuration.DefineParameters(&FPCParameters, "fpc")
	configuration.DefineParameters(&StatementParameters, "statement")
}
