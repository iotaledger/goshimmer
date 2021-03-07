package ledgerstate

import (
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAliasBasic(t *testing.T) {
	bals := NewColoredBalances(map[Color]uint64{ColorIOTA: 10})
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	out, err := NewAliasOutputMint(bals, addr, nil, nil)
	require.NoError(t, err)

	t.Logf("%s", out)
}

func TestAliasMint(t *testing.T) {
	bals1 := NewColoredBalances(map[Color]uint64{ColorIOTA: 10})
	_, err := NewAliasOutputMint(bals1, nil, nil, nil)
	require.Error(t, err)

	bals0 := NewColoredBalances(nil)
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	_, err = NewAliasOutputMint(bals0, addr, nil, nil)
	require.Error(t, err)

	out, err := NewAliasOutputMint(bals1, addr, nil, nil)
	require.NoError(t, err)

	bigData := make([]byte, MaxMetadataSize+1)
	_, err = NewAliasOutputMint(bals0, addr, bigData, nil)
	require.Error(t, err)

	out, err = NewAliasOutputMint(bals1, addr, nil, addr)
	require.NoError(t, err)

	t.Logf("%s", out)
}

func TestAliasDestroy(t *testing.T) {
	bals1 := NewColoredBalances(map[Color]uint64{ColorIOTA: 10})
	_, err := NewAliasOutputMint(bals1, nil, nil, nil)
	require.Error(t, err)

	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	state := TransactionID{}.Bytes()
	out, err := NewAliasOutputMint(bals1, addr, state, nil)
	require.NoError(t, err)

	out = out.NewAliasOutputDestroy()
	t.Logf("%s", out.String())
}
