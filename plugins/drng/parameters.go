package drng

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgDRNGInstanceID defines the config flag of the DRNG instanceID.
	CfgDRNGInstanceID = "drng.instanceId"
	// CfgDRNGThreshold defines the config flag of the DRNG threshold.
	CfgDRNGThreshold = "drng.threshold"
	// CfgDRNGDistributedPubKey defines the config flag of the DRNG distributed Public Key.
	CfgDRNGDistributedPubKey = "drng.distributedPubKey"
	// CfgDRNGCommitteeMembers defines the config flag of the DRNG committee members identities.
	CfgDRNGCommitteeMembers = "drng.committeeMembers"
)

func init() {
	flag.Uint32(CfgDRNGInstanceID, 1, "instance ID of the drng instance")
	flag.Uint32(CfgDRNGThreshold, 3, "BLS threshold of the drng")
	flag.String(CfgDRNGDistributedPubKey, "", "distributed public key of the committee (hex encoded)")
	flag.StringSlice(CfgDRNGCommitteeMembers, []string{}, "list of committee members of the drng")
}
