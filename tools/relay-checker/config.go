package main

import "github.com/iotaledger/goshimmer/plugins/config"

var (
	nodes        []string
	target       = ""
	msgData      = "TEST99BROADCAST99DATA"
	cooldownTime = 2
	repeat       = 1
)

func initConfig() {
	if config.Node().String(CfgTargetNode) == "" {
		panic("Set the target node address\n")
	}
	target = config.Node().String(CfgTargetNode)

	if len(config.Node().Strings(CfgTestNodes)) == 0 {
		panic("Set node addresses\n")
	}
	nodes = append(nodes, config.Node().Strings(CfgTestNodes)...)

	// optional settings
	if config.Node().String(CfgData) != "" {
		msgData = config.Node().String(CfgData)
	}
	if config.Node().Int(CfgCooldownTime) > 0 {
		cooldownTime = config.Node().Int(CfgCooldownTime)
	}
	if config.Node().Int(CfgRepeat) > 0 {
		repeat = config.Node().Int(CfgRepeat)
	}
}
