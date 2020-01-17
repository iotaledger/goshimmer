package main

import (
	"github.com/iotaledger/goshimmer/packages/parameter"
)

var (
	target   = ""
	neighbor = ""
	txnAddr  = "GOSHIMMER99TEST999999999999999999999999999999999999999999999999999999999999999999"
	txnData  = "TEST99BROADCAST99DATA"
	cooldown = 2
	maxQuery = 1
)

func LoadConfig() {
	if err := parameter.FetchConfig(true); err != nil {
		panic(err)
	}
}

func SetConfig() {
	if parameter.NodeConfig.GetString("relaycheck.targetNode") != "" {
		target = parameter.NodeConfig.GetString("relaycheck.targetNode")
	} else {
		panic("Set the target node address\n")
	}
	if parameter.NodeConfig.GetString("relaycheck.neighbor") != "" {
		neighbor = parameter.NodeConfig.GetString("relaycheck.neighbor")
	} else {
		panic("Set the neighbor node address\n")
	}
	// optional settings
	if parameter.NodeConfig.GetString("relaycheck.txnaddress") != "" {
		txnAddr = parameter.NodeConfig.GetString("relaycheck.txnaddress")
	}
	if parameter.NodeConfig.GetString("relaycheck.neighbor") != "" {
		txnData = parameter.NodeConfig.GetString("relaycheck.data")
	}
	if parameter.NodeConfig.GetInt("relaycheck.cooldowntime") > 0 {
		cooldown = parameter.NodeConfig.GetInt("relaycheck.cooldowntime")
	}
	if parameter.NodeConfig.GetInt("relaycheck.maxquery") > 0 {
		maxQuery = parameter.NodeConfig.GetInt("relaycheck.maxquery")
	}
}
