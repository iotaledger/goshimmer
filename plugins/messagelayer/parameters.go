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
	Listen        bool  `default:"true" usage:"if the FPC service should listen"`
	RoundInterval int64 `default:"10" usage:"FPC round interval [s]"`
}{}

func init() {
	configuration.DefineParameters(&Parameters, "messageLayer")
	configuration.DefineParameters(&FPCParameters, "fpc")

	pflag.PrintDefaults()
}
