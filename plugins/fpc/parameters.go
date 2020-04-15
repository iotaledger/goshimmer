package fpc

import (
	flag "github.com/spf13/pflag"
)

const (
	CfgFPCQuerySampleSize = "fpc.querySampleSize"
	CfgFPCRoundInterval   = "fpc.roundInterval"
	CfgFPCBindAddress     = "fpc.bindAddress"
)

func init() {
	flag.Int(CfgFPCQuerySampleSize, 3, "Size of the voting quorum (k)")
	flag.Int(CfgFPCRoundInterval, 5, "FPC round interval [s]")
	flag.String(CfgFPCBindAddress, "0.0.0.0:10895", "the bind address on which the FPC vote server binds to")
}
