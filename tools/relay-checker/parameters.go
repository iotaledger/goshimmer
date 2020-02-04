package main

import (
	flag "github.com/spf13/pflag"
)

const (
	CFG_TARGET_NODE   = "relayChecker.targetNode"
	CFG_TEST_NODES    = "relayChecker.testNodes"
	CFG_TX_ADDRESS    = "relayChecker.txAddress"
	CFG_DATA          = "relayChecker.data"
	CFG_COOLDOWN_TIME = "relayChecker.cooldownTime"
	CFG_REPEAT        = "relayChecker.repeat"
)

func init() {
	flag.StringSlice(CFG_TEST_NODES, []string{""}, "the list of nodes to check after the cooldown")
	flag.String(CFG_TARGET_NODE, "http://127.0.0.1:8080", "the target node from the which transaction will be broadcasted from")
	flag.String(CFG_TX_ADDRESS, "SHIMMER99TEST99999999999999999999999999999999999999999999999999999999999999999999", "the transaction address")
	flag.String(CFG_DATA, "TEST99BROADCAST99DATA", "data to broadcast")
	flag.Int(CFG_COOLDOWN_TIME, 10, "the cooldown time after broadcasting the data on the specified target node")
	flag.Int(CFG_REPEAT, 1, "the amount of times to repeat the relay-checker queries")
}
