package ledgerstate

import (
	"bytes"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAliasMint(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	_, err := NewChainOutputMint(bals1, nil)
	require.Error(t, err)

	bals0 := map[Color]uint64{}
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	_, err = NewChainOutputMint(bals0, addr)
	require.Error(t, err)

	out, err := NewChainOutputMint(bals1, addr)
	require.NoError(t, err)

	bigData := make([]byte, MaxOutputPayloadSize+1)
	out, err = NewChainOutputMint(bals1, addr)
	require.NoError(t, err)
	err = out.SetStateData(bigData)
	require.Error(t, err)

	out, err = NewChainOutputMint(bals1, addr)
	require.NoError(t, err)
	require.True(t, out.IsSelfGoverned())
	out.SetGoverningAddress(addr)
	require.True(t, out.IsSelfGoverned())

	t.Logf("%s", out)
}

func TestMarshal1(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	out, err := NewChainOutputMint(bals1, addr)
	require.NoError(t, err)

	data := out.Bytes()
	outBack, _, err := OutputFromBytes(data)
	require.NoError(t, err)

	require.EqualValues(t, AliasAddressType, outBack.Type())
	_, ok := outBack.(*ChainOutput)
	require.True(t, ok)

	require.Zero(t, out.Compare(outBack))
}

func TestMarshal2(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	kp1 := ed25519.GenerateKeyPair()
	addr1 := NewED25519Address(kp1.PublicKey)
	kp2 := ed25519.GenerateKeyPair()
	addr2 := NewED25519Address(kp2.PublicKey)

	out, err := NewChainOutputMint(bals1, addr1)
	require.NoError(t, err)
	require.True(t, out.IsSelfGoverned())
	out.SetGoverningAddress(addr2)
	require.False(t, out.IsSelfGoverned())

	data := out.Bytes()
	outBack, _, err := OutputFromBytes(data)
	require.NoError(t, err)

	require.EqualValues(t, AliasAddressType, outBack.Type())
	_, ok := outBack.(*ChainOutput)
	require.True(t, ok)

	require.Zero(t, out.Compare(outBack))
}

func TestMarshal3(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	out, err := NewChainOutputMint(bals1, addr)
	out.SetGoverningAddress(addr)
	require.NoError(t, err)
	err = out.SetStateData([]byte("dummy..."))
	require.NoError(t, err)

	data := out.Bytes()
	outBack, _, err := OutputFromBytes(data)
	require.NoError(t, err)

	require.EqualValues(t, AliasAddressType, outBack.Type())
	_, ok := outBack.(*ChainOutput)
	require.True(t, ok)

	require.Zero(t, out.Compare(outBack))
}

func TestStateTransition1(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	out, err := NewChainOutputMint(bals1, addr)
	require.NoError(t, err)

	oid := NewOutputID(TransactionID{}, 42)
	out.SetID(oid)

	outNext := out.NewChainOutputNext(true)

	require.Zero(t, bytes.Compare(out.GetAliasAddress().Bytes(), outNext.GetAliasAddress().Bytes()))
	outNext1, _, err := OutputFromBytes(outNext.Bytes())
	require.NoError(t, err)
	require.Zero(t, outNext.Compare(outNext1))
	require.Zero(t, bytes.Compare(outNext.Bytes(), outNext1.Bytes()))
}

func TestExtendedOutput(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)

	out := NewExtendedLockedOutput(bals1, addr)
	outBack, _, err := OutputFromBytes(out.Bytes())
	require.NoError(t, err)
	require.Zero(t, outBack.Compare(out))
}
