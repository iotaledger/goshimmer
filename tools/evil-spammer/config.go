package main

import (
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"time"
)

// Nodes used during the test, use at least two nodes to be able to doublespend
var (
	//urls = []string{"http://bootstrap-01.feature.shimmer.iota.cafe:8080", "http://vanilla-01.feature.shimmer.iota.cafe:8080", "http://drng-01.feature.shimmer.iota.cafe:8080"}
	urls = []string{"http://localhost:8080", "http://localhost:8090"}
)

var (
	Script = "basic"

	customSpamParams = CustomSpamParams{
		ClientUrls:            urls,
		SpamTypes:             []string{"msg"},
		Rates:                 []int{1},
		Durations:             []time.Duration{time.Second * 20},
		MsgToBeSent:           []int{0},
		TimeUnit:              time.Second,
		DelayBetweenConflicts: 0,
		Scenario:              evilwallet.Scenario1(),
		DeepSpam:              false,
	}
	quickTest = QuickTestParams{
		ClientUrls:            urls,
		Rate:                  100,
		Duration:              time.Second * 30,
		TimeUnit:              time.Second,
		DelayBetweenConflicts: 0,
	}
)
