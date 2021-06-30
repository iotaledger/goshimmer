package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLowerCamelCase(t *testing.T) {
	tests := []*struct {
		s, exp string
	}{
		{s: "", exp: ""},
		{s: "foo", exp: "foo"},
		{s: "FOO", exp: "foo"},
		{s: "fooBar", exp: "fooBar"},
		{s: "FooBar", exp: "fooBar"},
		{s: "FOOBar", exp: "fooBar"},
		{s: "FooBAR", exp: "fooBAR"},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			require.Equal(t, tt.exp, lowerCamelCase(tt.s))
		})
	}
}

func TestCreateFlags(t *testing.T) {
	var config GoShimmer
	config.POW.Enabled = true
	config.POW.Difficulty = 10
	assert.Contains(t, config.CreateFlags(), "--pow.difficulty=10")
}
