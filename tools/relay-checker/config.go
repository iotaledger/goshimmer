package main

import (
	"time"
)

var (
	nodes        []string
	target       = ""
	msgData      = "TEST99BROADCAST99DATA"
	cooldownTime = 2 * time.Second
	repeat       = 1
)

func initConfig() {
	if Parameters.TargetNode == "" {
		panic("Set the target node address\n")
	}
	target = Parameters.CfgTargetNode

	if len(Parameters.CfgTestNodes) == 0 {
		panic("Set node addresses\n")
	}
	nodes = append(nodes, Parameters.CfgTestNodes...)

	// optional settings
	if Parameters.CfgData != "" {
		msgData = Parameters.CfgData
	}
	if Parameters.CfgCooldownTime > 0 {
		cooldownTime = time.Duration(Parameters.CfgCooldownTime) * time.Second
	}
	if Parameters.CfgRepeat > 0 {
		repeat = Parameters.CfgRepeat
	}
}
