package main

import (
	"time"
)

// Nodes used during the test, use at least two nodes to be able to doublespend
var (
	//urls = []string{"http://bootstrap-01.feature.shimmer.iota.cafe:8080", "http://vanilla-01.feature.shimmer.iota.cafe:8080", "http://drng-01.feature.shimmer.iota.cafe:8080"}
	urls = []string{"http://localhost:8080", "http://localhost:8090"}
)

var (
	Script = "quick"

	customSpamParams = CustomSpamParams{
		ClientUrls:            urls,
		SpamTypes:             []string{"msg"},
		Rates:                 []int{100},
		DurationsInSec:        []int{60},
		MsgToBeSent:           []int{},
		TimeUnit:              time.Second,
		DelayBetweenConflicts: 0,
	}
	quickTest = QuickTestParams{
		ClientUrls:            urls,
		Rate:                  100,
		Duration:              time.Second * 30,
		TimeUnit:              time.Second,
		DelayBetweenConflicts: time.Millisecond * 100,
	}

	increasingDoubleSpendTest = IncreasingDoubleSpendTestParams{
		ClientUrls:            urls,
		Start:                 10,
		Stop:                  40,
		Step:                  10,
		EpochDuration:         time.Minute,
		TimeUnit:              time.Second,
		DelayBetweenConflicts: time.Millisecond * 100,
	}

	reasonableDoubleSpendTest = ReasonableDoubleSpendTestParams{
		ClientUrls:            urls,
		DataRate:              60,
		DsRate:                6,
		TimeUnit:              time.Hour,
		EpochDuration:         time.Hour,
		EpochsNumber:          5,
		DelayBetweenConflicts: time.Millisecond * 100,
	}
)
