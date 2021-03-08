package ledgerstate

import (
	"bytes"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAliasMint(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	_, err := NewAliasOutputMint(bals1, nil, nil, nil)
	require.Error(t, err)

	bals0 := map[Color]uint64{}
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	_, err = NewAliasOutputMint(bals0, addr, nil, nil)
	require.Error(t, err)

	out, err := NewAliasOutputMint(bals1, addr, nil, nil)
	require.NoError(t, err)

	bigData := make([]byte, MaxOutputPayloadSize+1)
	_, err = NewAliasOutputMint(bals0, addr, bigData, nil)
	require.Error(t, err)

	out, err = NewAliasOutputMint(bals1, addr, nil, addr)
	require.NoError(t, err)

	t.Logf("%s", out)
}

func TestMarshal1(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	out, err := NewAliasOutputMint(bals1, addr, nil, nil)
	require.NoError(t, err)

	data := out.Bytes()
	outBack, _, err := OutputFromBytes(data)
	require.NoError(t, err)

	require.EqualValues(t, AliasAddressType, outBack.Type())
	_, ok := outBack.(*AliasOutput)
	require.True(t, ok)

	require.Zero(t, out.Compare(outBack))
}

func TestMarshal2(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	out, err := NewAliasOutputMint(bals1, addr, nil, addr)
	require.NoError(t, err)

	data := out.Bytes()
	outBack, _, err := OutputFromBytes(data)
	require.NoError(t, err)

	require.EqualValues(t, AliasAddressType, outBack.Type())
	_, ok := outBack.(*AliasOutput)
	require.True(t, ok)

	require.Zero(t, out.Compare(outBack))
}

func TestMarshal3(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	out, err := NewAliasOutputMint(bals1, addr, []byte("dummy..."), addr)
	require.NoError(t, err)

	data := out.Bytes()
	outBack, _, err := OutputFromBytes(data)
	require.NoError(t, err)

	require.EqualValues(t, AliasAddressType, outBack.Type())
	_, ok := outBack.(*AliasOutput)
	require.True(t, ok)

	require.Zero(t, out.Compare(outBack))
}

func TestStateTransition1(t *testing.T) {
	bals1 := map[Color]uint64{ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := NewED25519Address(kp.PublicKey)
	out, err := NewAliasOutputMint(bals1, addr, nil, nil)
	require.NoError(t, err)

	oid := NewOutputID(TransactionID{}, 42)
	out.SetID(oid)

	outNext, err := out.NewAliasOutputStateTransition(out.Balances().Map(), out.GetStateData())
	require.NoError(t, err)

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
