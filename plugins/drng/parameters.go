package drng

import (
	flag "github.com/spf13/pflag"
)

const (
	CfgDRNGInstanceID        = "drng.instanceId"
	CfgDRNGThreshold         = "drng.threshold"
	CfgDRNGDistributedPubKey = "drng.distributedPubKey"
	CfgDRNGCommitteeMembers  = "drng.committeeMembers"
)

func init() {
	flag.Uint32(CfgDRNGInstanceID, 1, "instance ID of the drng instance")
	flag.Uint32(CfgDRNGThreshold, 3, "BLS threshold of the drng")
	flag.String(CfgDRNGDistributedPubKey, "", "distributed public key of the committee (hex encoded)")
	flag.StringSlice(CfgDRNGCommitteeMembers, []string{}, "list of committee members of the drng")
}
