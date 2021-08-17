package main

var (
	nodes        []string
	target       = ""
	msgData      = "TEST99BROADCAST99DATA"
	cooldownTime = 2
	repeat       = 1
)

func initConfig() {
	if deps.Config.String(CfgTargetNode) == "" {
		panic("Set the target node address\n")
	}
	target = deps.Config.String(CfgTargetNode)

	if len(deps.Config.Strings(CfgTestNodes)) == 0 {
		panic("Set node addresses\n")
	}
	nodes = append(nodes, deps.Config.Strings(CfgTestNodes)...)

	// optional settings
	if deps.Config.String(CfgData) != "" {
		msgData = deps.Config.String(CfgData)
	}
	if deps.Config.Int(CfgCooldownTime) > 0 {
		cooldownTime = deps.Config.Int(CfgCooldownTime)
	}
	if deps.Config.Int(CfgRepeat) > 0 {
		repeat = deps.Config.Int(CfgRepeat)
	}
}
