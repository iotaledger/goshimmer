package mana

import flag "github.com/spf13/pflag"

const (
	// CfgEmaCoefficient1 defines the coefficient used for Effective Base Mana 1 (moving average) calculation.
	CfgEmaCoefficient1 = "mana.emaCoefficient1"
	// CfgEmaCoefficient2 defines the coefficient used for Effective Base Mana 2 (moving average) calculation.
	CfgEmaCoefficient2 = "mana.emaCoefficient2"
	// CfgDecay defines the decay coefficient used for Base Mana 2 calculation.
	CfgDecay = "mana.decay"
	// CfgAllowedAccessPledge defines the list of nodes that access mana is allowed to be pledged to.
	CfgAllowedAccessPledge = "mana.allowedAccessPledge"
	// CfgAllowedAccessFilterEnabled defines if access mana pledge filter is enabled.
	CfgAllowedAccessFilterEnabled = "mana.allowedAccessFilterEnabled"
	// CfgAllowedConsensusPledge defines the list of nodes that consensus mana is allowed to be pledged to.
	CfgAllowedConsensusPledge = "mana.allowedConsensusPledge"
	// CfgAllowedConsensusFilterEnabled defines if consensus mana pledge filter is enabled.
	CfgAllowedConsensusFilterEnabled = "mana.allowedConsensusFilterEnabled"
)

func init() {
	flag.Float64(CfgEmaCoefficient1, 0.00003209, "coefficient used for Effective Base Mana 1 (moving average) calculation")
	flag.Float64(CfgEmaCoefficient2, 0.00003209, "coefficient used for Effective Base Mana 2 (moving average) calculation")
	flag.Float64(CfgDecay, 0.00003209, "decay coefficient used for Base Mana 2 calculation")
	flag.StringSlice(CfgAllowedAccessPledge, nil, "list of nodes that access mana is allowed to be pledged to")
	flag.StringSlice(CfgAllowedConsensusPledge, nil, "list of nodes that consensus mana is allowed to be pledge to")
	flag.Bool(CfgAllowedAccessFilterEnabled, false, "if filtering on access mana pledge nodes is enabled")
	flag.Bool(CfgAllowedConsensusFilterEnabled, false, "if filtering on consensus mana pledge nodes is enabled")
}
