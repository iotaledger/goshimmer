package messagelayer

import (
	"github.com/iotaledger/goshimmer/packages/configuration"
	"github.com/spf13/pflag"
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

func init() {
	configuration.DefineParameters(&Parameters, "messageLayer")
	configuration.DefineParameters(&FPCParameters, "fpc")

	pflag.PrintDefaults()
}
