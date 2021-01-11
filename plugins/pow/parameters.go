package pow

import (
	"time"

	flag "github.com/spf13/pflag"
)

const (
	// CfgPOWDifficulty defines the config flag of the PoW difficulty.
	CfgPOWDifficulty = "pow.difficulty"
	// CfgPOWNumThreads defines the config flag of the number of threads used to do the PoW.
	CfgPOWNumThreads = "pow.numThreads"
	// CfgPOWTimeout defines the config flag for the PoW timeout.
	CfgPOWTimeout = "pow.timeout"
	// CfgPOWWindow defines the config flag for the aPoW window.
	CfgPOWWindow = "pow.window"
	// CfgPOWRate defines the config flag for the aPoW rate.
	CfgPOWRate = "pow.rate"
)

func init() {
	flag.Int(CfgPOWDifficulty, 22, "PoW difficulty")
	flag.Int(CfgPOWNumThreads, 1, "number of threads used to do the PoW")
	flag.Duration(CfgPOWTimeout, time.Minute, "PoW timeout")
	flag.Int(CfgPOWWindow, 60, "number of seconds used to define the aPoW window")
	flag.Float64(CfgPOWRate, 1/6, "rate of the aPoW")
}
