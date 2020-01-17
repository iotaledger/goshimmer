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
	if parameter.NodeConfig.GetString("relaycheck.targetNode") == "" {
		panic("Set the target node address\n")
	}
	target = parameter.NodeConfig.GetString("relaycheck.targetNode")

	if len(parameter.NodeConfig.GetStringSlice("relaycheck.nodes")) == 0 {
		panic("Set node addresses\n")
	}
	nodes = append(nodes, parameter.NodeConfig.GetStringSlice("relaycheck.nodes")...)

	// optional settings
	if parameter.NodeConfig.GetString("relaycheck.txnaddress") != "" {
		txnAddr = parameter.NodeConfig.GetString("relaycheck.txnaddress")
	}
	if parameter.NodeConfig.GetString("relaycheck.data") != "" {
		txnData = parameter.NodeConfig.GetString("relaycheck.data")
	}
	if parameter.NodeConfig.GetInt("relaycheck.cooldowntime") > 0 {
		cooldown = parameter.NodeConfig.GetInt("relaycheck.cooldowntime")
	}
	if parameter.NodeConfig.GetInt("relaycheck.maxquery") > 0 {
		maxQuery = parameter.NodeConfig.GetInt("relaycheck.maxquery")
	}
}
