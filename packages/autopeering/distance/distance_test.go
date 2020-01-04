package distance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBySalt(t *testing.T) {
	type testCase struct {
		x            []byte
		y            []byte
		salt         []byte
		zeroDistance bool
	}

	tests := []testCase{
		{
			x:            []byte("X"),
			y:            []byte("Y"),
			salt:         []byte("salt"),
			zeroDistance: false,
		},
		{
			x:            []byte("X"),
			y:            []byte("X"),
			salt:         []byte{},
			zeroDistance: true,
		},
	}

	for _, test := range tests {
		d := BySalt(test.x, test.y, test.salt)
		got := d == 0
		assert.Equal(t, test.zeroDistance, got, "Zero Distance")
	}

}
