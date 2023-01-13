package chat

import (
	"testing"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/stretchr/testify/require"
)

func TestPayload(t *testing.T) {
	a := NewPayload("me", "you", "ciao")
	abytes := lo.PanicOnErr(a.Bytes())
	b := new(Payload)
	_, err := b.FromBytes(abytes)
	require.NoError(t, err)
	require.Equal(t, a, b)
}
