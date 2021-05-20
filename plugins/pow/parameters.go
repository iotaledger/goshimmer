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
	// CfgPOWParentsRefreshInterval defines the config flag for the PoW parents refresh interval.
	CfgPOWParentsRefreshInterval = "pow.parentsRefreshInterval"
	parentsRefreshRateDefault    = 300 * time.Millisecond
)

func init() {
	flag.Int(CfgPOWDifficulty, 22, "PoW difficulty")
	flag.Int(CfgPOWNumThreads, 1, "number of threads used to do the PoW")
	flag.Duration(CfgPOWTimeout, time.Minute, "PoW timeout")
	flag.Duration(CfgPOWParentsRefreshInterval, parentsRefreshRateDefault, "PoW parents refresh interval timeout")
}
