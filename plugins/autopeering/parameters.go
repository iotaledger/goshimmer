package autopeering

import "github.com/iotaledger/hive.go/configuration"

// Parameters contains the configuration parameters used by the message layer.
var Parameters = struct {
	// Mana defines the config flag of mana in the autopeering.
	Mana bool `default:"true" usage:"enable/disable mana in the autopeering"`

	// R defines the config flag of R.
	R int `default:"40" usage:"R parameter"`

	// Ro defines the config flag of Ro.
	Ro float64 `default:"2." usage:"Ro parameter"`
}{}

func init() {
	configuration.BindParameters(&Parameters, "autopeering")
}
