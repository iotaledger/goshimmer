package pow

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the pow plugin.
type ParametersDefinition struct {
	// Difficulty defines the PoW difficulty.
	Difficulty int `default:"21" usage:"PoW difficulty"`
	// NumThreads defines how many threaded workers are used to do PoW.
	NumThreads int `default:"1" usage:"number of threads used to do the PoW"`
	// Timeout defines the maximum allow time to perform PoW.
	Timeout time.Duration `default:"1m" usage:"PoW timeout"`
	// ParentsRefreshInterval defines the timeout for parents refreshing.
	ParentsRefreshInterval time.Duration `default:"300ms" usage:"PoW parents refresh interval timeout"`
}

// Parameters contains the configuration used by the pow plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "pow")
}
