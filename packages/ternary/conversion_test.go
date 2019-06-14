package ternary

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestInt64Conversion(t *testing.T) {
	assert.Equal(t, Int64ToTrits(9223372036854775807).ToInt64(), int64(9223372036854775807))
	assert.Equal(t, Int64ToTrits(1337).ToInt64(), int64(1337))
	assert.Equal(t, Int64ToTrits(0).ToInt64(), int64(0))
	assert.Equal(t, Int64ToTrits(-1337).ToInt64(), int64(-1337))
	assert.Equal(t, Int64ToTrits(-9223372036854775808).ToInt64(), int64(-9223372036854775808))
}

func TestUInt64Conversion(t *testing.T) {
	assert.Equal(t, Uint64ToTrits(18446744073709551615).ToUint64(), uint64(18446744073709551615))
	assert.Equal(t, Uint64ToTrits(1337).ToUint64(), uint64(1337))
	assert.Equal(t, Uint64ToTrits(0).ToUint64(), uint64(0))
}
