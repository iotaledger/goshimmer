package manarefresher

import "github.com/iotaledger/hive.go/configuration"

// Parameters contains the configuration parameters used by the manarefresher plugin.
var Parameters = struct {
	RefreshInterval uint `default:"25" usage:"interval for refreshing delegated mana (minutes)"`
}{}

func init() {
	configuration.BindParameters(&Parameters, "manarefresher")
}
