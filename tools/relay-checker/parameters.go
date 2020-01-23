package main

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_TARGET_NODE    = "relaycheck.targetNode"
	CFG_TEST_NODES     = "relaycheck.nodes"
	CFG_TXN_ADDRESS    = "relaycheck.txnAddress"
	CFG_DATA           = "relaycheck.data"
	CFG_COOL_DOWN_TIME = "relaycheck.cooldownTime"
	CFG_MAX_QUERY      = "relaycheck.maxQuery"
)

func init() {
	flag.StringSlice(CFG_TEST_NODES, []string{""}, "list of trusted entry nodes for auto peering")
	flag.String(CFG_TARGET_NODE, "http://127.0.0.1:8080", "target node to test")
	flag.String(CFG_TXN_ADDRESS, "SHIMMER99TEST99999999999999999999999999999999999999999999999999999999999999999999", "transaction address")
	flag.String(CFG_DATA, "TEST99BROADCAST99DATA", "data to broadcast")
	flag.Int(CFG_COOL_DOWN_TIME, 10, "cooldown time after broadcast data")
	flag.Int(CFG_MAX_QUERY, 1, "the repeat times of relay-checker")
}
