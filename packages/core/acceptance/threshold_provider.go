package acceptance

import (
	"math"

	"github.com/iotaledger/hive.go/lo"
)

const bftThreshold = 0.67

func ThresholdProvider(totalWeightProvider func() int64) func() int64 {
	return func() int64 {
		// TODO: should we allow threshold go to 0? or should acceptance stop if no committee member is active?
		return lo.Max(int64(math.Ceil(float64(totalWeightProvider())*bftThreshold)), 1)
	}
}
