package constants

import (
	"math"
	"time"
)

// MinInt64 is the smallest number representable as int64.
const MinInt64 = -math.MaxInt64 - 1

var (
	// MaxRepresentableTime is the greatest time that doesn't overflow when converted to unix nanoseconds.
	MaxRepresentableTime = time.Unix(0, math.MaxInt64)
	// MinRepresentableTime is the smallest time that doesn't overflow when converted to unix nanoseconds.
	MinRepresentableTime = time.Unix(0, MinInt64)
)
