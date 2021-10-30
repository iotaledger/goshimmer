package main

import (
	"time"

	"github.com/iotaledger/hive.go/configuration"
)

// ParametersDefinition contains the definition of configuration parameters used by the relaychecker.
type ParametersDefinition struct {
	// TargetNode defines the target node.
	TargetNode string `default:"http://127.0.0.1:8080" usage:"the target node from the which message will be broadcasted from"`
	// TestNodes defines test nodes.
	TestNodes []string `usage:"the list of nodes to check after the cooldown"`
	// CfgData defines the data.
	Data string `default:"TEST99BROADCAST99DATA" usage:"data to broadcast"`
	// CooldownTime defines the cooldown time.
	CooldownTime time.Duration `default:"10s" usage:"the cooldown time after broadcasting the data on the specified target node"`
	// CfgRepeat defines the repeat.
	Repeat int `default:"1" usage:"the amount of times to repeat the relay-checker queries"`
}

// Parameters contains the configuration parameters of the relay checker.
var Parameters = &ParametersDefinition{}

func init() {
	configuration.BindParameters(Parameters, "relayChecker")
}
