package autopeering

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgMana defines the config flag of mana in the autopeering.
	CfgMana = "autopeering.mana"
	// CfgR defines the config flag of R.
	CfgR = "autopeering.R"
	// CfgRo defines the config flag of Ro.
	CfgRo = "autopeeringRo"
)

func init() {
	flag.Bool(CfgMana, true, "Enable/disable mana in the autopeering")
	flag.Int(CfgR, 40, "R parameter")
	flag.Float64(CfgRo, 2., "Ro parameter")
}
