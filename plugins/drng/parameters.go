package drng

import (
	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the drng plugin.
type ParametersDefinition struct {
	// Pollen contains the configuration parameters of GoShimmer dRNG committee.
	Pollen struct {
		// InstanceID defines the config flag of the DRNG instanceId.
		InstanceID int `name:"instanceId" default:"1" usage:"instance ID of the GoShimmer drng instance"`

		// Threshold defines the config flag of the DRNG threshold.
		Threshold int `default:"3" usage:"BLS threshold of the GoShimmer drng"`

		// DistributedPubKey defines the config flag of the DRNG distributed Public Key.
		DistributedPubKey string `usage:"distributed public key of the GoShimmer committee (hex encoded)"`

		// CommitteeMembers defines the config flag of the DRNG committee members identities.
		CommitteeMembers []string `usage:"list of committee members of the GoShimmer drng"`
	} `name:"pollen"`

	// XTeam contains the configuration parameters of the X-Team dRNG committee.
	XTeam struct {
		// InstanceID defines the config flag of the DRNG instanceId.
		InstanceID int `name:"instanceId" default:"1339" usage:"instance ID of the x-team drng instance"`

		// Threshold defines the config flag of the DRNG threshold.
		Threshold int `default:"3" usage:"BLS threshold of the x-team drng"`

		// DistributedPubKey defines the config flag of the DRNG distributed Public Key.
		DistributedPubKey string `usage:"distributed public key of the x-team committee (hex encoded)"`

		// CommitteeMembers defines the config flag of the DRNG committee members identities.
		CommitteeMembers []string `usage:"list of committee members of the x-team drng"`
	} `name:"xteam"`

	// Custom contains the configuration parameters of the custom dRNG committee.
	Custom struct {
		// InstanceID defines the config flag of the DRNG instanceId.
		InstanceID int `name:"instanceId" default:"9999" usage:"instance ID of the custom drng instance"`

		// Threshold defines the config flag of the DRNG threshold.
		Threshold int `default:"3" usage:"BLS threshold of the custom drng"`

		// DistributedPubKey defines the config flag of the DRNG distributed Public Key.
		DistributedPubKey string `usage:"distributed public key of the custom committee (hex encoded)"`

		// CommitteeMembers defines the config flag of the DRNG committee members identities.
		CommitteeMembers []string `usage:"list of committee members of the custom drng"`
	} `name:"custom"`
}

// Parameters contains the configuration parameters of the drng plugin.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "drng")
}
