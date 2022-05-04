package notarization

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the notarization plugin.
type ParametersDefinition struct {
	// MinEpochCommitableDuration defines the min age of a commitable epoch.
	MinEpochCommitableDuration time.Duration `default:"24m" usage:"min age of a commitable epoch"`
}

// Parameters contains the configuration used by the networkdelay plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "notarization")
}
