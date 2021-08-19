package main

import "github.com/iotaledger/hive.go/configuration"

// ParametersDefinition contains the definition of configuration parameters used by the relaychecker.
type ParametersDefinition struct {
	// TargetNode defines the target node.
	TargetNode []string `usage:"the list of nodes to check after the cooldown"`
	// TestNodes defines test nodes.
	TestNodes string `default:"http://127.0.0.1:8080" usage:"the target node from the which message will be broadcasted from"`
	// CfgData defines the data.
	Data string `default:"TEST99BROADCAST99DATA" usage:"data to broadcast"`
	// CooldownTime defines the cooldown time.
	CooldownTime int `default:"10" usage:"the cooldown time after broadcasting the data on the specified target node"`
	// CfgRepeat defines the repeat.
	Repeat int `default:"1" usage:"the amount of times to repeat the relay-checker queries"`
}

// ParametersNetwork contains the configuration parameters of the local peer's network.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "relayChecker")
}
