package blockissuer

import (
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
)

// ParametersDefinition contains the definition of configuration parameters used by the p2p plugin.
type ParametersDefinition struct {
	// RateSetter contains the definition of the parameters used by the Rate Setter.
	RateSetter struct {
		// Mode determines the type of the rate setting mechanism.
		Mode string `default:"deficit" usage:"define the type of rate setter to use. Possible options are deficit, aimd or disabled."`
		// Initial defines the initial rate of rate setting.
		Initial float64 `default:"1" usage:"the initial rate of AIMD rate setting (if in AIMD mode). Set 0 to automatically estimate the value based on access mana."`
		// Pause defines for how long to pause updates after decrease of rate.
		Pause time.Duration `default:"1s" usage:"for how long to pause updates after decrease of rate (if in AIMD mode)."`
	}

	// RateSetter contains the definition of the parameters used by the Rate Setter.
	BlockFactory struct {
		// TipSelectionTimeout defines after how much time to stop trying to select tips.
		TipSelectionTimeout time.Duration `default:"10s" usage:"after how much time to stop trying to select tips"`
		// TipSelectionRetryInterval defines how much time to sleep after a failed tip selection attempt.
		TipSelectionRetryInterval time.Duration `default:"200ms" usage:"how much time to sleep after a failed tip selection attempt"`
	}

	IgnoreBootstrappedFlag bool `default:"false" usage:"whether to ignore bootstrapped flag and issue blocks regardless"`
}

// Parameters contains the configuration parameters of the p2p plugin.
var Parameters = &ParametersDefinition{}

func init() {
	config.BindParameters(Parameters, "blockIssuer")
}
