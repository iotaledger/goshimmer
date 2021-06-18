package config

import (
	"log"
	"testing"

	"github.com/mr-tron/base58"
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

	log.Println(base58.Decode("2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5"))
}

func TestCreateFlags(t *testing.T) {
	config := GoShimmer{
		POW: POW{true, 10},
	}
	assert.Contains(t, config.CreateFlags(), "--pow.difficulty=10")
}
