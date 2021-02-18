package ledgerstate

import (
	"math"
)

func SafeAddUint64(a uint64, b uint64) (result uint64, valid bool) {
	valid = !(math.MaxUint64 - a < b)
	result = a + b
	return
}

func SafeSubUint64(a uint64, b uint64) (result uint64, valid bool) {
	valid = b <= a
	result = a - b
	return
}