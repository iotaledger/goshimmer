package drng

import (
	flag "github.com/spf13/pflag"
)

const (
	// Configuration parameters of Pollen dRNG committee.

	// CfgDRNGInstanceID defines the config flag of the DRNG instanceID.
	CfgDRNGInstanceID = "drng.pollen.instanceId"
	// CfgDRNGThreshold defines the config flag of the DRNG threshold.
	CfgDRNGThreshold = "drng.pollen.threshold"
	// CfgDRNGDistributedPubKey defines the config flag of the DRNG distributed Public Key.
	CfgDRNGDistributedPubKey = "drng.pollen.distributedPubKey"
	// CfgDRNGCommitteeMembers defines the config flag of the DRNG committee members identities.
	CfgDRNGCommitteeMembers = "drng.pollen.committeeMembers"

	// Configuration parameters of X-Team dRNG committee.

	// CfgDRNGXTeamInstanceID defines the config flag of the DRNG instanceID.
	CfgDRNGXTeamInstanceID = "drng.xteam.instanceId"
	// CfgDRNGXTeamThreshold defines the config flag of the DRNG threshold.
	CfgDRNGXTeamThreshold = "drng.xteam.threshold"
	// CfgDRNGXTeamDistributedPubKey defines the config flag of the DRNG distributed Public Key.
	CfgDRNGXTeamDistributedPubKey = "drng.xteam.distributedPubKey"
	// CfgDRNGXTeamCommitteeMembers defines the config flag of the DRNG committee members identities.
	CfgDRNGXTeamCommitteeMembers = "drng.xteam.committeeMembers"

	// Configuration parameters of Custom dRNG committee.

	// CfgDRNGCustomInstanceID defines the config flag of the DRNG instanceID.
	CfgDRNGCustomInstanceID = "drng.custom.instanceId"
	// CfgDRNGCustomThreshold defines the config flag of the DRNG threshold.
	CfgDRNGCustomThreshold = "drng.custom.threshold"
	// CfgDRNGCustomDistributedPubKey defines the config flag of the DRNG distributed Public Key.
	CfgDRNGCustomDistributedPubKey = "drng.custom.distributedPubKey"
	// CfgDRNGCustomCommitteeMembers defines the config flag of the DRNG committee members identities.
	CfgDRNGCustomCommitteeMembers = "drng.custom.committeeMembers"
)

func init() {
	// Default parameters of Pollen dRNG committee.
	flag.Int(CfgDRNGInstanceID, Pollen, "instance ID of the pollen drng instance")
	flag.Int(CfgDRNGThreshold, 3, "BLS threshold of the pollen drng")
	flag.String(CfgDRNGDistributedPubKey, "", "distributed public key of the pollen committee (hex encoded)")
	flag.StringSlice(CfgDRNGCommitteeMembers, []string{}, "list of committee members of the pollen drng")

	// Default parameters of X-Team dRNG committee.
	flag.Int(CfgDRNGXTeamInstanceID, XTeam, "instance ID of the x-team drng instance")
	flag.Int(CfgDRNGXTeamThreshold, 3, "BLS threshold of the x-team drng")
	flag.String(CfgDRNGXTeamDistributedPubKey, "", "distributed public key of the x-team committee (hex encoded)")
	flag.StringSlice(CfgDRNGXTeamCommitteeMembers, []string{}, "list of committee members of the x-team drng")

	// Default parameters of Custom dRNG committee.
	flag.Int(CfgDRNGCustomInstanceID, 9999, "instance ID of the custom drng instance")
	flag.Int(CfgDRNGCustomThreshold, 3, "BLS threshold of the custom drng")
	flag.String(CfgDRNGCustomDistributedPubKey, "", "distributed public key of the custom committee (hex encoded)")
	flag.StringSlice(CfgDRNGCustomCommitteeMembers, []string{}, "list of committee members of the custom drng")
}
