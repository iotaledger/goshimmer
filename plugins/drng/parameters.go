package drng

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_INSTANCE_ID         = "drng.instanceId"
	CFG_THRESHOLD           = "drng.threshold"
	CFG_DISTRIBUTED_PUB_KEY = "drng.distributedPubKey"
	CFG_COMMITTEE_MEMBERS   = "drng.committeeMembers"
)

func init() {
	flag.Uint32(CFG_INSTANCE_ID, 1, "instance ID of the drng instance")
	flag.Uint32(CFG_THRESHOLD, 3, "BLS threshold of the drng")
	flag.String(CFG_DISTRIBUTED_PUB_KEY, "", "distributed public key of the committee (hex encoded)")
	flag.StringSlice(CFG_COMMITTEE_MEMBERS, []string{}, "list of committee members of the drng")
}
