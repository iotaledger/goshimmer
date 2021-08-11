package manarefresher

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of the parameters used by the manarefresher plugin.
type ParametersDefinition struct {
	// RefreshInterval defines the interval for refreshing delegated mana.
	RefreshInterval time.Duration `default:"25m" usage:"interval for refreshing delegated mana (minutes)"`
}

// Parameters contains the configuration used by the manarefresher plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "manarefresher")
}
