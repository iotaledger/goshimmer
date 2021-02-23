package ledgerstate

import (
	"math"
)

// SafeAddUint64 adds two uint64 values. It returns the result and a valid flag that indicates whether the addition is
// valid without causing an overflow.
func SafeAddUint64(a uint64, b uint64) (result uint64, valid bool) {
	valid = !(math.MaxUint64-a < b)
	result = a + b
	return
}

// SafeSubUint64 subtracts two uint64 values. It returns the result and a valid flag that indicates whether the
// subtraction is valid without causing an overflow.
func SafeSubUint64(a uint64, b uint64) (result uint64, valid bool) {
	valid = b <= a
	result = a - b
	return
}
