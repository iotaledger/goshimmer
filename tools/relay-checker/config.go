package main

import (
	"github.com/iotaledger/goshimmer/packages/parameter"
)

var (
	nodes    []string
	target   = ""
	txnAddr  = "GOSHIMMER99TEST999999999999999999999999999999999999999999999999999999999999999999"
	txnData  = "TEST99BROADCAST99DATA"
	cooldown = 2
	maxQuery = 1
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
	if parameter.NodeConfig.GetString(CFG_TXN_ADDRESS) != "" {
		txnAddr = parameter.NodeConfig.GetString(CFG_TXN_ADDRESS)
	}
	if parameter.NodeConfig.GetString(CFG_DATA) != "" {
		txnData = parameter.NodeConfig.GetString(CFG_DATA)
	}
	if parameter.NodeConfig.GetInt(CFG_COOL_DOWN_TIME) > 0 {
		cooldown = parameter.NodeConfig.GetInt(CFG_COOL_DOWN_TIME)
	}
	if parameter.NodeConfig.GetInt(CFG_MAX_QUERY) > 0 {
		maxQuery = parameter.NodeConfig.GetInt(CFG_MAX_QUERY)
	}
}
