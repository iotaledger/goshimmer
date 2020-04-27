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
	if config.Node.GetString(CfgTargetNode) == "" {
		panic("Set the target node address\n")
	}
	target = config.Node.GetString(CfgTargetNode)

	if len(config.Node.GetStringSlice(CfgTestNodes)) == 0 {
		panic("Set node addresses\n")
	}
	nodes = append(nodes, config.Node.GetStringSlice(CfgTestNodes)...)

	// optional settings
	if config.Node.GetString(CfgData) != "" {
		msgData = config.Node.GetString(CfgData)
	}
	if config.Node.GetInt(CfgCooldownTime) > 0 {
		cooldownTime = config.Node.GetInt(CfgCooldownTime)
	}
	if config.Node.GetInt(CfgRepeat) > 0 {
		repeat = config.Node.GetInt(CfgRepeat)
	}
}
