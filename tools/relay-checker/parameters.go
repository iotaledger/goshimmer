package main

import (
	flag "github.com/spf13/pflag"
)

const (
	// CfgTargetNode defines the config flag of the target node.
	CfgTargetNode = "relayChecker.targetNode"
	// CfgTestNodes defines the config flag of the test nodes.
	CfgTestNodes = "relayChecker.testNodes"
	// CfgData defines the config flag of the data.
	CfgData = "relayChecker.data"
	// CfgCooldownTime defines the config flag of the cooldown time.
	CfgCooldownTime = "relayChecker.cooldownTime"
	// CfgRepeat defines the config flag of the repeat.
	CfgRepeat = "relayChecker.repeat"
)

func init() {
	flag.StringSlice(CfgTargetNode, []string{""}, "the list of nodes to check after the cooldown")
	flag.String(CfgTestNodes, "http://127.0.0.1:8080", "the target node from the which message will be broadcasted from")
	flag.String(CfgData, "TEST99BROADCAST99DATA", "data to broadcast")
	flag.Int(CfgCooldownTime, 10, "the cooldown time after broadcasting the data on the specified target node")
	flag.Int(CfgRepeat, 1, "the amount of times to repeat the relay-checker queries")
}
