package main

import (
	"github.com/iotaledger/goshimmer/packages/parameter"
)

var (
	nodes        []string
	target       = ""
	txnAddr      = "GOSHIMMER99TEST999999999999999999999999999999999999999999999999999999999999999999"
	txnData      = "TEST99BROADCAST99DATA"
	cooldownTime = 2
	repeat       = 1
)

func LoadConfig() {
	if err := parameter.FetchConfig(false); err != nil {
		panic(err)
	}
}

func SetConfig() {
	if parameter.NodeConfig.GetString(CFG_TARGET_NODE) == "" {
		panic("Set the target node address\n")
	}
	target = parameter.NodeConfig.GetString(CFG_TARGET_NODE)

	if len(parameter.NodeConfig.GetStringSlice(CFG_TEST_NODES)) == 0 {
		panic("Set node addresses\n")
	}
	nodes = append(nodes, parameter.NodeConfig.GetStringSlice(CFG_TEST_NODES)...)

	// optional settings
	if parameter.NodeConfig.GetString(CFG_TX_ADDRESS) != "" {
		txnAddr = parameter.NodeConfig.GetString(CFG_TX_ADDRESS)
	}
	if parameter.NodeConfig.GetString(CFG_DATA) != "" {
		txnData = parameter.NodeConfig.GetString(CFG_DATA)
	}
	if parameter.NodeConfig.GetInt(CFG_COOLDOWN_TIME) > 0 {
		cooldownTime = parameter.NodeConfig.GetInt(CFG_COOLDOWN_TIME)
	}
	if parameter.NodeConfig.GetInt(CFG_REPEAT) > 0 {
		repeat = parameter.NodeConfig.GetInt(CFG_REPEAT)
	}
}
