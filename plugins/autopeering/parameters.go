package autopeering

import "github.com/iotaledger/hive.go/configuration"

// ParametersDefinition contains the definition of configuration parameters used by the autopeering plugin.
type ParametersDefinition struct {
	// Mana defines the config flag of mana in the autopeering.
	Mana bool `default:"true" usage:"enable/disable mana in the autopeering"`

	// R defines the config flag of R.
	R int `default:"40" usage:"R parameter"`

	// Ro defines the config flag of Ro.
	Ro float64 `default:"2." usage:"Ro parameter"`

	EnableGossipIntegration bool `default:"true" usage:"enable/disable autopeering for gossip layer"`
}

// Parameters contains the configuration parameters of the autopeering plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "autopeering")
}
