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
	if len(Parameters.TargetNode) == 0 {
		panic("Set the target node address\n")
	}
	target = Parameters.TargetNode[0]

	if Parameters.TestNodes == "" {
		panic("Set node addresses\n")
	}
	nodes = append(nodes, Parameters.TestNodes)

	// optional settings
	if Parameters.Data != "" {
		msgData = Parameters.Data
	}
	if Parameters.CooldownTime > 0 {
		cooldownTime = time.Duration(Parameters.CooldownTime) * time.Second
	}
	if Parameters.Repeat > 0 {
		repeat = Parameters.Repeat
	}
}
