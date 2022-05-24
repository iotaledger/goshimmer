package utxotest

import (
	"bytes"
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func TestAliasOutputMint(t *testing.T) {
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	_, err := ledgerstate.NewAliasOutputMint(bals1, nil)
	require.Error(t, err)

	bals0 := map[ledgerstate.Color]uint64{}
	kp := ed25519.GenerateKeyPair()
	addr := ledgerstate.NewED25519Address(kp.PublicKey)
	_, err = ledgerstate.NewAliasOutputMint(bals0, addr)
	require.Error(t, err)

	_, err = ledgerstate.NewAliasOutputMint(bals1, addr)
	require.NoError(t, err)

	bigData := make([]byte, ledgerstate.MaxOutputPayloadSize+1)
	out, err := ledgerstate.NewAliasOutputMint(bals1, addr)
	require.NoError(t, err)
	err = out.SetStateData(bigData)
	require.Error(t, err)

	out, err = ledgerstate.NewAliasOutputMint(bals1, addr)
	require.NoError(t, err)
	require.True(t, out.IsSelfGoverned())
	out.SetGoverningAddress(addr)
	require.True(t, out.IsSelfGoverned())

	out, err = ledgerstate.NewAliasOutputMint(bals1, addr, []byte("dummy"))
	require.NoError(t, err)
	require.True(t, out.IsSelfGoverned())
	out.SetGoverningAddress(addr)
	require.True(t, out.IsSelfGoverned())
	require.EqualValues(t, "dummy", string(out.GetImmutableData()))

	t.Logf("%s", out)
}

func TestAliasOutputMarshal1(t *testing.T) {
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := ledgerstate.NewED25519Address(kp.PublicKey)
	out, err := ledgerstate.NewAliasOutputMint(bals1, addr)
	require.NoError(t, err)

	data := out.Bytes()
	outBack, err := ledgerstate.OutputFromBytes(data)
	require.NoError(t, err)

	require.EqualValues(t, ledgerstate.AliasAddressType, outBack.Type())
	_, ok := outBack.(*ledgerstate.AliasOutput)
	require.True(t, ok)

	require.Zero(t, out.Compare(outBack))
}

func TestAliasOutputMarshal2(t *testing.T) {
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	kp1 := ed25519.GenerateKeyPair()
	addr1 := ledgerstate.NewED25519Address(kp1.PublicKey)
	kp2 := ed25519.GenerateKeyPair()
	addr2 := ledgerstate.NewED25519Address(kp2.PublicKey)

	out, err := ledgerstate.NewAliasOutputMint(bals1, addr1)
	require.NoError(t, err)
	require.True(t, out.IsSelfGoverned())
	out.SetGoverningAddress(addr2)
	require.False(t, out.IsSelfGoverned())

	data := out.Bytes()
	outBack, err := ledgerstate.OutputFromBytes(data)
	require.NoError(t, err)

	require.EqualValues(t, ledgerstate.AliasAddressType, outBack.Type())
	_, ok := outBack.(*ledgerstate.AliasOutput)
	require.True(t, ok)

	require.Zero(t, out.Compare(outBack))
}

func TestAliasOutputMarshal3(t *testing.T) {
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := ledgerstate.NewED25519Address(kp.PublicKey)
	out, err := ledgerstate.NewAliasOutputMint(bals1, addr)
	out.SetGoverningAddress(addr)
	require.NoError(t, err)
	err = out.SetStateData([]byte("dummy..."))
	require.NoError(t, err)

	data := out.Bytes()
	outBack, err := ledgerstate.OutputFromBytes(data)
	require.NoError(t, err)

	require.EqualValues(t, ledgerstate.AliasAddressType, outBack.Type())
	_, ok := outBack.(*ledgerstate.AliasOutput)
	require.True(t, ok)

	require.Zero(t, out.Compare(outBack))
}

func TestAliasOutputMarshal4(t *testing.T) {
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := ledgerstate.NewED25519Address(kp.PublicKey)
	out, err := ledgerstate.NewAliasOutputMint(bals1, addr, []byte("dummy NFT..."))
	out.SetGoverningAddress(addr)
	require.NoError(t, err)
	err = out.SetStateData([]byte("dummy state..."))
	require.NoError(t, err)

	data := out.Bytes()
	outBack, err := ledgerstate.OutputFromBytes(data)
	require.NoError(t, err)

	require.EqualValues(t, ledgerstate.AliasAddressType, outBack.Type())
	_, ok := outBack.(*ledgerstate.AliasOutput)
	require.True(t, ok)

	require.Zero(t, out.Compare(outBack))
}

func TestStateTransition1(t *testing.T) {
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := ledgerstate.NewED25519Address(kp.PublicKey)
	out, err := ledgerstate.NewAliasOutputMint(bals1, addr)
	require.NoError(t, err)

	oid := ledgerstate.NewOutputID(ledgerstate.TransactionID{}, 42)
	out.SetID(oid)
	out.SetAliasAddress(out.GetAliasAddress())

	outNext := out.NewAliasOutputNext()

	require.Zero(t, bytes.Compare(out.GetAliasAddress().Bytes(), outNext.GetAliasAddress().Bytes()))
	outNext1, err := ledgerstate.OutputFromBytes(outNext.Bytes())
	require.NoError(t, err)
	require.Zero(t, outNext.Compare(outNext1))
	require.Zero(t, bytes.Compare(outNext.Bytes(), outNext1.Bytes()))
}

func TestStateTransition2(t *testing.T) {
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := ledgerstate.NewED25519Address(kp.PublicKey)
	out, err := ledgerstate.NewAliasOutputMint(bals1, addr, []byte("dummy NFT"))
	require.NoError(t, err)

	oid := ledgerstate.NewOutputID(ledgerstate.TransactionID{}, 42)
	out.SetID(oid)
	out.SetAliasAddress(out.GetAliasAddress())

	outNext := out.NewAliasOutputNext()

	require.Zero(t, bytes.Compare(out.GetAliasAddress().Bytes(), outNext.GetAliasAddress().Bytes()))
	outNext1, err := ledgerstate.OutputFromBytes(outNext.Bytes())
	require.NoError(t, err)
	require.Zero(t, outNext.Compare(outNext1))
	require.Zero(t, bytes.Compare(outNext.Bytes(), outNext1.Bytes()))
	require.EqualValues(t, "dummy NFT", string(outNext.GetImmutableData()))
}

func TestExtendedOutput(t *testing.T) {
	bals1 := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: 100}
	kp := ed25519.GenerateKeyPair()
	addr := ledgerstate.NewED25519Address(kp.PublicKey)

	out := ledgerstate.NewExtendedLockedOutput(bals1, addr)
	outBack, err := ledgerstate.OutputFromBytes(out.Bytes())
	require.NoError(t, err)
	require.Zero(t, outBack.Compare(out))
}
