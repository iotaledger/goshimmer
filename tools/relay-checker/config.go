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

func LoadConfig() {
	if err := config.Fetch(false); err != nil {
		panic(err)
	}
}

func SetConfig() {
	if config.NodeConfig.GetString(CFG_TARGET_NODE) == "" {
		panic("Set the target node address\n")
	}
	target = config.NodeConfig.GetString(CFG_TARGET_NODE)

	if len(config.NodeConfig.GetStringSlice(CFG_TEST_NODES)) == 0 {
		panic("Set node addresses\n")
	}
	nodes = append(nodes, config.NodeConfig.GetStringSlice(CFG_TEST_NODES)...)

	// optional settings
	if config.NodeConfig.GetString(CFG_TX_ADDRESS) != "" {
		txnAddr = config.NodeConfig.GetString(CFG_TX_ADDRESS)
	}
	if config.NodeConfig.GetString(CFG_DATA) != "" {
		txnData = config.NodeConfig.GetString(CFG_DATA)
	}
	if config.NodeConfig.GetInt(CFG_COOLDOWN_TIME) > 0 {
		cooldownTime = config.NodeConfig.GetInt(CFG_COOLDOWN_TIME)
	}
	if config.NodeConfig.GetInt(CFG_REPEAT) > 0 {
		repeat = config.NodeConfig.GetInt(CFG_REPEAT)
	}
}
