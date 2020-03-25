package main

import "github.com/iotaledger/goshimmer/plugins/config"

var (
	nodes        []string
	target       = ""
	txnAddr      = "GOSHIMMER99TEST999999999999999999999999999999999999999999999999999999999999999999"
	txnData      = "TEST99BROADCAST99DATA"
	cooldownTime = 2
	repeat       = 1
)

func InitConfig() {
	if config.Node.GetString(CFG_TARGET_NODE) == "" {
		panic("Set the target node address\n")
	}
	target = config.Node.GetString(CFG_TARGET_NODE)

	if len(config.Node.GetStringSlice(CFG_TEST_NODES)) == 0 {
		panic("Set node addresses\n")
	}
	nodes = append(nodes, config.Node.GetStringSlice(CFG_TEST_NODES)...)

	// optional settings
	if config.Node.GetString(CFG_TX_ADDRESS) != "" {
		txnAddr = config.Node.GetString(CFG_TX_ADDRESS)
	}
	if config.Node.GetString(CFG_DATA) != "" {
		txnData = config.Node.GetString(CFG_DATA)
	}
	if config.Node.GetInt(CFG_COOLDOWN_TIME) > 0 {
		cooldownTime = config.Node.GetInt(CFG_COOLDOWN_TIME)
	}
	if config.Node.GetInt(CFG_REPEAT) > 0 {
		repeat = config.Node.GetInt(CFG_REPEAT)
	}
}
