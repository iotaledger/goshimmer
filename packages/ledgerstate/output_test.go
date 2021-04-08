package ledgerstate

import (
	"bytes"
	"encoding/binary"
	"github.com/stretchr/testify/require"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"
)

// region AliasOutput Tests

func TestAliasOutputIsOrigin(t *testing.T) {
	tokens := map[Color]uint64{ColorIOTA: 200}
	pub, _, err := ed25519.GenerateKey()
	require.NoError(t, err)
	addr := NewED25519Address(pub)
	out, err := NewAliasOutputMint(tokens, addr)
	require.NoError(t, err)
	require.True(t, out.IsOrigin())
	out.SetID(OutputID{})
	outUpd := out.UpdateMintingColor().(*AliasOutput)
	require.True(t, outUpd.IsOrigin())
}

func TestAliasOutput_NewAliasOutputMint(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		stateAddy := randEd25119Address()
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, stateAddy)
		assert.NoError(t, err)
		iotaBal, ok := alias.Balances().Get(ColorIOTA)
		assert.True(t, ok)
		assert.Equal(t, DustThresholdAliasOutputIOTA, iotaBal)
		assert.True(t, alias.GetStateAddress().Equals(stateAddy))
		assert.Nil(t, alias.GetImmutableData())
	})

	t.Run("CASE: Happy path with immutable data", func(t *testing.T) {
		stateAddy := randEd25119Address()
		data := []byte("dummy")
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, stateAddy, data)
		assert.NoError(t, err)
		iotaBal, ok := alias.Balances().Get(ColorIOTA)
		assert.True(t, ok)
		assert.Equal(t, DustThresholdAliasOutputIOTA, iotaBal)
		assert.True(t, alias.GetStateAddress().Equals(stateAddy))
		assert.True(t, bytes.Equal(alias.GetImmutableData(), data))
	})

	t.Run("CASE: Below dust threshold", func(t *testing.T) {
		stateAddy := randEd25119Address()
		data := []byte("dummy")
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA - 1}, stateAddy, data)
		assert.Error(t, err)
		assert.Nil(t, alias)
	})

	t.Run("CASE: State address is an alias", func(t *testing.T) {
		stateAddy := randAliasAddress()
		data := []byte("dummy")
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, stateAddy, data)
		assert.Error(t, err)
		assert.Nil(t, alias)
	})

	t.Run("CASE: Non existent state address", func(t *testing.T) {
		data := []byte("dummy")
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, nil, data)
		assert.Error(t, err)
		assert.Nil(t, alias)
	})

	t.Run("CASE: Too big state data", func(t *testing.T) {
		stateAddy := randAliasAddress()
		data := make([]byte, MaxOutputPayloadSize+1)
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, stateAddy, data)
		assert.Error(t, err)
		assert.Nil(t, alias)
	})
}

func TestAliasOutput_NewAliasOutputNext(t *testing.T) {
	originAlias := dummyAliasOutput()

	t.Run("CASE: Happy path, no governance update", func(t *testing.T) {
		nextAlias := originAlias.NewAliasOutputNext()
		assert.True(t, originAlias.GetAliasAddress().Equals(nextAlias.GetAliasAddress()))
		assert.True(t, originAlias.GetStateAddress().Equals(nextAlias.GetStateAddress()))
		assert.True(t, originAlias.GetGoverningAddress().Equals(nextAlias.GetGoverningAddress()))
		// outputid is actually irrelevant here
		assert.True(t, bytes.Equal(nextAlias.ID().Bytes(), originAlias.ID().Bytes()))
		assert.Equal(t, originAlias.Balances().Bytes(), nextAlias.Balances().Bytes())
		assert.Equal(t, originAlias.GetStateIndex()+1, nextAlias.GetStateIndex())
		assert.Equal(t, originAlias.GetStateData(), nextAlias.GetStateData())
		assert.Equal(t, originAlias.GetImmutableData(), nextAlias.GetImmutableData())
		assert.Equal(t, originAlias.GetIsGovernanceUpdated(), nextAlias.GetIsGovernanceUpdated())
	})

	t.Run("CASE: Happy path, governance update", func(t *testing.T) {
		nextAlias := originAlias.NewAliasOutputNext(true)
		assert.True(t, originAlias.GetAliasAddress().Equals(nextAlias.GetAliasAddress()))
		assert.True(t, originAlias.GetStateAddress().Equals(nextAlias.GetStateAddress()))
		assert.True(t, originAlias.GetGoverningAddress().Equals(nextAlias.GetGoverningAddress()))
		// outputid is actually irrelevant here
		assert.True(t, bytes.Equal(nextAlias.ID().Bytes(), originAlias.ID().Bytes()))
		assert.Equal(t, originAlias.Balances().Bytes(), nextAlias.Balances().Bytes())
		assert.Equal(t, originAlias.GetStateIndex(), nextAlias.GetStateIndex())
		assert.Equal(t, originAlias.GetStateData(), nextAlias.GetStateData())
		assert.Equal(t, originAlias.GetImmutableData(), nextAlias.GetImmutableData())
		assert.NotEqual(t, originAlias.GetIsGovernanceUpdated(), nextAlias.GetIsGovernanceUpdated())
	})

	t.Run("CASE: Previous was governance update, next is not", func(t *testing.T) {
		originAlias = dummyAliasOutput()
		// previous output was a governance update
		originAlias.SetIsGovernanceUpdated(true)
		nextAlias := originAlias.NewAliasOutputNext()
		// created output should not be a governance update
		assert.False(t, nextAlias.GetIsGovernanceUpdated())
		assert.True(t, originAlias.GetAliasAddress().Equals(nextAlias.GetAliasAddress()))
		assert.True(t, originAlias.GetStateAddress().Equals(nextAlias.GetStateAddress()))
		assert.True(t, originAlias.GetGoverningAddress().Equals(nextAlias.GetGoverningAddress()))
		// outputid is actually irrelevant here
		assert.True(t, bytes.Equal(nextAlias.ID().Bytes(), originAlias.ID().Bytes()))
		assert.Equal(t, originAlias.Balances().Bytes(), nextAlias.Balances().Bytes())
		assert.Equal(t, originAlias.GetStateIndex()+1, nextAlias.GetStateIndex())
		assert.Equal(t, originAlias.GetStateData(), nextAlias.GetStateData())
		assert.Equal(t, originAlias.GetImmutableData(), nextAlias.GetImmutableData())
	})

	t.Run("CASE: Previous was governance update, next as well", func(t *testing.T) {
		originAlias = dummyAliasOutput()
		// previous output was a governance update
		originAlias.SetIsGovernanceUpdated(true)
		nextAlias := originAlias.NewAliasOutputNext(true)
		// created output should be a governance update
		assert.True(t, nextAlias.GetIsGovernanceUpdated())
		assert.True(t, originAlias.GetAliasAddress().Equals(nextAlias.GetAliasAddress()))
		assert.True(t, originAlias.GetStateAddress().Equals(nextAlias.GetStateAddress()))
		assert.True(t, originAlias.GetGoverningAddress().Equals(nextAlias.GetGoverningAddress()))
		// outputid is actually irrelevant here
		assert.True(t, bytes.Equal(nextAlias.ID().Bytes(), originAlias.ID().Bytes()))
		assert.Equal(t, originAlias.Balances().Bytes(), nextAlias.Balances().Bytes())
		assert.Equal(t, originAlias.GetStateIndex(), nextAlias.GetStateIndex())
		assert.Equal(t, originAlias.GetStateData(), nextAlias.GetStateData())
		assert.Equal(t, originAlias.GetImmutableData(), nextAlias.GetImmutableData())
	})
}

func TestAliasOutputFromMarshalUtil(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		bytesLength := len(originAlias.Bytes())
		marshaledAlias, consumed, err := OutputFromBytes(originAlias.Bytes())
		assert.NoError(t, err)
		assert.Equal(t, bytesLength, consumed)
		assert.Equal(t, marshaledAlias.Bytes(), originAlias.Bytes())
	})

	t.Run("CASE: Wrong type", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		originBytes := originAlias.Bytes()
		// manually change output type byte
		originBytes[0] = 1
		marshalUtil := marshalutil.New(originBytes)
		_, err := AliasOutputFromMarshalUtil(marshalUtil)
		assert.Error(t, err)
	})

	t.Run("CASE: Wrong flag for state data", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		originBytes := originAlias.Bytes()
		flags := originAlias.mustFlags()
		flags = flags.ClearBit(flagAliasOutputStateDataPresent)
		// manually change flags
		originBytes[1] = byte(flags)
		_, _, err := OutputFromBytes(originBytes)
		assert.Error(t, err)
	})

	t.Run("CASE: Wrong flag for immutable data", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		originBytes := originAlias.Bytes()
		flags := originAlias.mustFlags()
		flags = flags.ClearBit(flagAliasOutputImmutableDataPresent)
		// manually change flags
		originBytes[1] = byte(flags)
		_, _, err := OutputFromBytes(originBytes)
		assert.Error(t, err)
	})

	t.Run("CASE: Wrong flag for governance address", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		originBytes := originAlias.Bytes()
		flags := originAlias.mustFlags()
		flags = flags.ClearBit(flagAliasOutputGovernanceSet)
		// manually change flags
		originBytes[1] = byte(flags)
		_, _, err := OutputFromBytes(originBytes)
		assert.Error(t, err)
	})

	t.Run("CASE: Flags provided, state data missing", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		// remove the data
		_ = originAlias.SetStateData(nil)
		originBytes := originAlias.Bytes()
		flags := originAlias.mustFlags()
		flags = flags.SetBit(flagAliasOutputStateDataPresent)
		// manually change flags
		originBytes[1] = byte(flags)
		_, _, err := OutputFromBytes(originBytes)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Flags provided, immutable data missing", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		// remove the data
		originAlias.SetImmutableData(nil)
		originBytes := originAlias.Bytes()
		flags := originAlias.mustFlags()
		flags = flags.SetBit(flagAliasOutputImmutableDataPresent)
		// manually change flags
		originBytes[1] = byte(flags)
		_, _, err := OutputFromBytes(originBytes)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Flags provided, governing address missing", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		// remove the data
		originAlias.SetGoverningAddress(originAlias.stateAddress)
		originBytes := originAlias.Bytes()
		flags := originAlias.mustFlags()
		flags = flags.SetBit(flagAliasOutputGovernanceSet)
		// manually change flags
		originBytes[1] = byte(flags)
		_, _, err := OutputFromBytes(originBytes)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Invalid balances", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		// remove the data
		invalidBalancesBytes := NewColoredBalances(map[Color]uint64{ColorIOTA: 99}).Bytes()
		originBytes := originAlias.Bytes()
		// serialized balances start at : output type (1 byte) + flags (1 byte) + AliasAddressLength bytes
		copy(originBytes[1+1+AddressLength:], invalidBalancesBytes)
		_, _, err := OutputFromBytes(originBytes)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Invalid state index for chain starting output", func(t *testing.T) {
		originAlias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randEd25119Address())
		assert.NoError(t, err)
		originBytes := originAlias.Bytes()
		stateIndexStartIndex := 1 + 1 + AddressLength + len(originAlias.balances.Bytes()) + AddressLength
		binary.LittleEndian.PutUint32(originBytes[stateIndexStartIndex:], 5)
		_, _, err = OutputFromBytes(originBytes)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Too much state data", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		originAlias.immutableData = nil
		originAlias.governingAddress = nil
		originAlias.stateData = []byte{1}
		originBytes := originAlias.Bytes()
		stateDataSizeIndex := 1 + 1 + AddressLength + len(originAlias.balances.Bytes()) + AddressLength + 4
		binary.LittleEndian.PutUint16(originBytes[stateDataSizeIndex:], MaxOutputPayloadSize+1)
		fakeStateData := make([]byte, MaxOutputPayloadSize)
		// original one byte state data is left untouched
		modBytes := byteutils.ConcatBytes(originBytes, fakeStateData)
		_, _, err := OutputFromBytes(modBytes)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Too much immutable data", func(t *testing.T) {
		originAlias := dummyAliasOutput()
		originAlias.immutableData = []byte{1}
		originAlias.governingAddress = nil
		originAlias.stateData = nil
		originBytes := originAlias.Bytes()
		immutableDataSizeIndex := 1 + 1 + AddressLength + len(originAlias.balances.Bytes()) + AddressLength + 4
		binary.LittleEndian.PutUint16(originBytes[immutableDataSizeIndex:], MaxOutputPayloadSize+1)
		fakeImmutableData := make([]byte, MaxOutputPayloadSize)
		// original one byte state data is left untouched
		modBytes := byteutils.ConcatBytes(originBytes, fakeImmutableData)
		_, _, err := OutputFromBytes(modBytes)
		t.Log(err)
		assert.Error(t, err)
	})
}

func TestAliasOutput_SetBalances(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		err := alias.SetBalances(map[Color]uint64{ColorIOTA: 1337})
		assert.NoError(t, err)
		cBalBytes := alias.Balances().Bytes()
		assert.Equal(t, NewColoredBalances(map[Color]uint64{ColorIOTA: 1337}).Bytes(), cBalBytes)
	})

	t.Run("CASE: Below threshold", func(t *testing.T) {
		alias := dummyAliasOutput()
		err := alias.SetBalances(map[Color]uint64{ColorIOTA: 99})
		assert.Error(t, err)
	})
}

func TestAliasOutput_SetAliasAddress(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		newAliAddy := randAliasAddress()
		alias.SetAliasAddress(newAliAddy)
		assert.True(t, alias.aliasAddress.Equals(newAliAddy))
	})
}

func TestAliasOutput_Balances(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		assert.Equal(t, NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}).Bytes(), alias.Balances().Bytes())
	})
}

func TestAliasOutput_Bytes(t *testing.T) {
	t.Run("Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		aBytes := alias.Bytes()
		mUtil := marshalutil.New(aBytes)
		restoredAlias, err := AliasOutputFromMarshalUtil(mUtil)
		assert.NoError(t, err)
		assert.True(t, alias.GetAliasAddress().Equals(restoredAlias.GetAliasAddress()))
		assert.True(t, alias.GetStateAddress().Equals(restoredAlias.GetStateAddress()))
		assert.True(t, alias.GetGoverningAddress().Equals(restoredAlias.GetGoverningAddress()))
		assert.Equal(t, alias.Balances().Bytes(), restoredAlias.Balances().Bytes())
		assert.Equal(t, alias.GetStateIndex(), restoredAlias.GetStateIndex())
		assert.Equal(t, alias.GetStateData(), restoredAlias.GetStateData())
		assert.Equal(t, alias.GetImmutableData(), restoredAlias.GetImmutableData())
		assert.Equal(t, alias.GetIsGovernanceUpdated(), restoredAlias.GetIsGovernanceUpdated())
	})
}

func TestAliasOutput_Compare(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		aBytes := alias.Bytes()
		mUtil := marshalutil.New(aBytes)
		restoredAlias, err := AliasOutputFromMarshalUtil(mUtil)
		assert.NoError(t, err)
		assert.True(t, alias.Compare(restoredAlias) == 0)
	})
}

func TestAliasOutput_GetGoverningAddress(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		governingAddy := alias.GetGoverningAddress()
		assert.True(t, governingAddy.Equals(alias.governingAddress))
	})

	t.Run("CASE: Self governed", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.governingAddress = nil
		governingAddy := alias.GetGoverningAddress()
		assert.True(t, governingAddy.Equals(alias.stateAddress))
	})
}

func TestAliasOutput_GetImmutableData(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		data := alias.GetImmutableData()
		assert.Equal(t, data, alias.immutableData)
	})

	t.Run("CASE: No data", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.immutableData = nil
		data := alias.GetImmutableData()
		assert.Equal(t, data, alias.immutableData)
	})
}

func TestAliasOutput_GetIsGovernanceUpdated(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		isGovUpdate := alias.GetIsGovernanceUpdated()
		assert.Equal(t, isGovUpdate, alias.isGovernanceUpdate)
	})

	t.Run("CASE: Happy path, false", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.isGovernanceUpdate = false
		isGovUpdate := alias.GetIsGovernanceUpdated()
		assert.Equal(t, isGovUpdate, alias.isGovernanceUpdate)
	})
}

func TestAliasOutput_GetStateAddress(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		stateAddy := alias.GetStateAddress()
		assert.True(t, stateAddy.Equals(alias.stateAddress))
	})
}

func TestAliasOutput_GetStateData(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		data := alias.GetStateData()
		assert.Equal(t, data, alias.stateData)
	})

	t.Run("CASE: No data", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.stateData = nil
		data := alias.GetStateData()
		assert.Equal(t, data, alias.stateData)
	})
}

func TestAliasOutput_GetStateIndex(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.stateIndex = 5
		sIndex := alias.GetStateIndex()
		assert.Equal(t, sIndex, alias.stateIndex)
	})
}

func TestAliasOutput_ID(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		id := alias.ID()
		assert.Equal(t, id, alias.outputID)
	})
}

func TestAliasOutput_Input(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		input := alias.Input()
		assert.Equal(t, input.Base58(), NewUTXOInput(alias.outputID).Base58())
	})

	t.Run("CASE: No output id yet", func(t *testing.T) {
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randEd25119Address())
		assert.NoError(t, err)
		assert.Panics(t, func() {
			_ = alias.Input()
		})
	})
}

func TestAliasOutput_IsOrigin(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		assert.False(t, alias.IsOrigin())
	})

	t.Run("CASE: Is origin", func(t *testing.T) {
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randEd25119Address())
		assert.NoError(t, err)
		assert.True(t, alias.IsOrigin())
	})
}

func TestAliasOutput_IsSelfGoverned(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		assert.False(t, alias.IsSelfGoverned())
	})

	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.governingAddress = nil
		assert.True(t, alias.IsSelfGoverned())
	})
}

func TestAliasOutput_ObjectStorageKey(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		assert.Equal(t, alias.outputID.Bytes(), alias.ObjectStorageKey())
	})
}

func TestAliasOutput_ObjectStorageValue(t *testing.T) {
	// same as Bytes()
}

func TestAliasOutput_SetGoverningAddress(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		newAddy := randAliasAddress()
		alias.SetGoverningAddress(newAddy)
		assert.True(t, alias.GetGoverningAddress().Equals(newAddy))
	})
}

func TestAliasOutput_SetID(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		newID := randOutputID()
		alias.SetID(newID)
		assert.Equal(t, alias.ID().Bytes(), newID.Bytes())
	})
}

func TestAliasOutput_SetImmutableData(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		data := []byte("new dummy nft data")
		alias.SetImmutableData(data)
		assert.Equal(t, alias.GetImmutableData(), data)
	})
}

func TestAliasOutput_SetIsGovernanceUpdated(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.SetIsGovernanceUpdated(true)
		assert.Equal(t, alias.GetIsGovernanceUpdated(), true)
	})
}

func TestAliasOutput_SetStateAddress(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		newWrongAddy := randAliasAddress()
		newAddy := randEd25119Address()
		err := alias.SetStateAddress(newWrongAddy)
		assert.Error(t, err)
		err = alias.SetStateAddress(newAddy)
		assert.NoError(t, err)
		assert.True(t, alias.GetStateAddress().Equals(newAddy))
	})
}

func TestAliasOutput_SetStateData(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		data := []byte("new dummy nft data")
		err := alias.SetStateData(data)
		assert.NoError(t, err)
		assert.Equal(t, alias.GetStateData(), data)
	})

	t.Run("CASE: Too much data", func(t *testing.T) {
		alias := dummyAliasOutput()
		data := make([]byte, MaxOutputPayloadSize+1)
		err := alias.SetStateData(data)
		assert.Error(t, err)
	})
}

func TestAliasOutput_SetStateIndex(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.SetStateIndex(5)
		assert.Equal(t, alias.GetStateIndex(), uint32(5))
	})
}

func TestAliasOutput_Type(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		assert.Equal(t, alias.Type(), AliasOutputType)
	})
}

func TestAliasOutput_Update(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		other := dummyAliasOutput()
		assert.Panics(t, func() {
			alias.Update(other)
		})
	})
}

func TestAliasOutput_UpdateMintingColor(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.balances = NewColoredBalances(map[Color]uint64{
			ColorIOTA: DustThresholdAliasOutputIOTA,
			ColorMint: 500,
		})
		updated := alias.UpdateMintingColor()
		balance, ok := updated.Balances().Get(blake2b.Sum256(alias.ID().Bytes()))
		assert.True(t, ok)
		assert.Equal(t, uint64(500), balance)
		assert.True(t, updated.Address().Equals(alias.GetAliasAddress()))
	})

	t.Run("CASE: No mint", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.balances = NewColoredBalances(map[Color]uint64{
			ColorIOTA: DustThresholdAliasOutputIOTA,
		})
		updated := alias.UpdateMintingColor()
		balance, ok := updated.Balances().Get(blake2b.Sum256(alias.ID().Bytes()))
		assert.False(t, ok)
		assert.Equal(t, uint64(0), balance)
		balance, ok = updated.Balances().Get(ColorIOTA)
		assert.True(t, ok)
		assert.Equal(t, DustThresholdAliasOutputIOTA, balance)
		assert.True(t, updated.Address().Equals(alias.GetAliasAddress()))
	})

	t.Run("CASE: Alias address is updated", func(t *testing.T) {
		alias := dummyAliasOutput(true)
		alias.aliasAddress = AliasAddress{}
		alias.balances = NewColoredBalances(map[Color]uint64{
			ColorIOTA: DustThresholdAliasOutputIOTA,
			ColorMint: 500,
		})
		updated := alias.UpdateMintingColor()
		balance, ok := updated.Balances().Get(blake2b.Sum256(alias.ID().Bytes()))
		assert.True(t, ok)
		assert.Equal(t, uint64(500), balance)
		assert.True(t, updated.Address().Equals(NewAliasAddress(alias.ID().Bytes())))
	})
}

func TestAliasOutput_validateTransition(t *testing.T) {
	t.Run("CASE: Happy path, state transition", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(false)
		err := prev.validateTransition(next)
		assert.NoError(t, err)
	})

	t.Run("CASE: Happy path, governance transition", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(true)
		err := prev.validateTransition(next)
		assert.NoError(t, err)
	})

	t.Run("CASE: Modified alias address", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(false)
		next.aliasAddress = *randAliasAddress()
		err := prev.validateTransition(next)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Modified immutable data", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(false)
		next.immutableData = []byte("something new")
		err := prev.validateTransition(next)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Gov update, modified state data", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(true)
		next.stateData = []byte("something new")
		err := prev.validateTransition(next)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Gov update, modified state index", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(true)
		next.stateIndex = prev.stateIndex + 1
		err := prev.validateTransition(next)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Gov update, modified balance", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(true)
		newBalance := prev.Balances().Map()
		newBalance[ColorIOTA]++
		next.balances = NewColoredBalances(newBalance)
		err := prev.validateTransition(next)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Gov update, modified state address", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(true)
		next.stateAddress = randEd25119Address()
		err := prev.validateTransition(next)
		assert.NoError(t, err)
	})

	t.Run("CASE: Gov update, modified governance address", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(true)
		next.governingAddress = randAliasAddress()
		err := prev.validateTransition(next)
		assert.NoError(t, err)
	})

	t.Run("CASE: State update, wrong state index", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(false)
		next.stateIndex = prev.GetStateIndex() + 2
		err := prev.validateTransition(next)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: State update, modify state address", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(false)
		next.stateAddress = randEd25119Address()
		err := prev.validateTransition(next)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: State update, modify governance address", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(false)
		next.governingAddress = randAliasAddress()
		err := prev.validateTransition(next)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: State update, was self governed", func(t *testing.T) {
		prev := dummyAliasOutput()
		prev.governingAddress = nil
		next := prev.NewAliasOutputNext(false)
		next.governingAddress = randAliasAddress()
		err := prev.validateTransition(next)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: State update, was not self governed", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(false)
		next.governingAddress = nil
		err := prev.validateTransition(next)
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: State update, modify state data", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(false)
		next.stateData = []byte("new state data")
		err := prev.validateTransition(next)
		assert.NoError(t, err)
	})

	t.Run("CASE: State update, modify balances", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := prev.NewAliasOutputNext(false)
		newBalance := prev.Balances().Map()
		newBalance[ColorIOTA]++
		next.balances = NewColoredBalances(newBalance)
		err := prev.validateTransition(next)
		assert.NoError(t, err)
	})
}

func TestAliasOutput_validateDestroyTransition(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		prev := dummyAliasOutput()
		err := prev.validateDestroyTransition()
		assert.NoError(t, err)
	})

	t.Run("CASE: More balance than minimum", func(t *testing.T) {
		prev := dummyAliasOutput()
		newBalance := prev.Balances().Map()
		newBalance[ColorIOTA]++
		prev.balances = NewColoredBalances(newBalance)
		err := prev.validateDestroyTransition()
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: More color balance than minimum", func(t *testing.T) {
		prev := dummyAliasOutput()
		newBalance := prev.Balances().Map()
		newBalance[Color{8}] = 1
		prev.balances = NewColoredBalances(newBalance)
		err := prev.validateDestroyTransition()
		t.Log(err)
		assert.Error(t, err)
	})

	t.Run("CASE: Only color balance", func(t *testing.T) {
		prev := dummyAliasOutput()
		prev.balances = NewColoredBalances(map[Color]uint64{{8}: DustThresholdAliasOutputIOTA})
		err := prev.validateDestroyTransition()
		t.Log(err)
		assert.Error(t, err)
	})
}

func TestAliasOutput_findChainedOutputAndCheckFork(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		prev := dummyAliasOutput()
		chained := prev.NewAliasOutputNext(false)
		outputs := Outputs{chained}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(prev.ID())), NewOutputs(outputs...))
		// unlockblocks are irrelevant now
		tx := NewTransaction(essence, UnlockBlocks{NewReferenceUnlockBlock(0)})

		found, err := prev.findChainedOutputAndCheckFork(tx)
		assert.NoError(t, err)
		assert.Equal(t, chained.Bytes(), found.Bytes())
	})

	t.Run("CASE: No alias output", func(t *testing.T) {
		prev := dummyAliasOutput()
		outputs := Outputs{NewSigLockedSingleOutput(DustThresholdAliasOutputIOTA, randEd25119Address())}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(prev.ID())), NewOutputs(outputs...))
		// unlockblocks are irrelevant now
		tx := NewTransaction(essence, UnlockBlocks{NewReferenceUnlockBlock(0)})

		// not found means that returned output is nil, no error
		found, _ := prev.findChainedOutputAndCheckFork(tx)
		assert.Nil(t, found)
	})

	t.Run("CASE: Duplicated alias output", func(t *testing.T) {
		prev := dummyAliasOutput()
		chained1 := prev.NewAliasOutputNext(false)
		chained2 := prev.NewAliasOutputNext(true)
		outputs := Outputs{chained1, chained2}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(prev.ID())), NewOutputs(outputs...))
		// unlockblocks are irrelevant now
		tx := NewTransaction(essence, UnlockBlocks{NewReferenceUnlockBlock(0)})

		found, err := prev.findChainedOutputAndCheckFork(tx)
		t.Log(err)
		assert.Error(t, err)
		assert.Nil(t, found)
	})

	t.Run("CASE: More than one alias in outputs", func(t *testing.T) {
		prev := dummyAliasOutput()
		chained := prev.NewAliasOutputNext(false)
		chainedFake := prev.NewAliasOutputNext(false)
		chainedFake.aliasAddress = *randAliasAddress()
		outputs := Outputs{chained, chainedFake}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(prev.ID())), NewOutputs(outputs...))
		// unlockblocks are irrelevant now
		tx := NewTransaction(essence, UnlockBlocks{NewReferenceUnlockBlock(0)})

		found, err := prev.findChainedOutputAndCheckFork(tx)
		assert.NoError(t, err)
		assert.Equal(t, chained.Bytes(), found.Bytes())
	})
}

func TestAliasOutput_hasToBeUnlockedForGovernanceUpdate(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		prev := dummyAliasOutput()
		chained := prev.NewAliasOutputNext(true)
		outputs := Outputs{chained}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(prev.ID())), NewOutputs(outputs...))
		// unlockblocks are irrelevant now
		tx := NewTransaction(essence, UnlockBlocks{NewReferenceUnlockBlock(0)})

		ok := prev.hasToBeUnlockedForGovernanceUpdate(tx)
		assert.True(t, ok)
	})

	t.Run("CASE: No governance update", func(t *testing.T) {
		prev := dummyAliasOutput()
		chained := prev.NewAliasOutputNext(false)
		outputs := Outputs{chained}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(prev.ID())), NewOutputs(outputs...))
		// unlockblocks are irrelevant now
		tx := NewTransaction(essence, UnlockBlocks{NewReferenceUnlockBlock(0)})

		ok := prev.hasToBeUnlockedForGovernanceUpdate(tx)
		assert.False(t, ok)
	})

	t.Run("CASE: Duplicated alias", func(t *testing.T) {
		prev := dummyAliasOutput()
		chained := prev.NewAliasOutputNext(true)
		chainedDuplicate := chained.clone()
		chainedDuplicate.stateData = []byte("duplicated")
		outputs := Outputs{chained, chainedDuplicate}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(prev.ID())), NewOutputs(outputs...))
		// unlockblocks are irrelevant now
		tx := NewTransaction(essence, UnlockBlocks{NewReferenceUnlockBlock(0)})

		ok := prev.hasToBeUnlockedForGovernanceUpdate(tx)
		assert.False(t, ok)
	})

	t.Run("CASE: No alias output found", func(t *testing.T) {
		prev := dummyAliasOutput()
		next := NewSigLockedSingleOutput(DustThresholdAliasOutputIOTA, randEd25119Address())
		outputs := Outputs{next}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(prev.ID())), NewOutputs(outputs...))
		// unlockblocks are irrelevant now
		tx := NewTransaction(essence, UnlockBlocks{NewReferenceUnlockBlock(0)})

		ok := prev.hasToBeUnlockedForGovernanceUpdate(tx)
		assert.True(t, ok)
	})
}

func TestAliasOutput_unlockedGovernanceByAliasIndex(t *testing.T) {
	governingAliasStateWallet := genRandomWallet()
	governingAlias := &AliasOutput{
		outputID:            randOutputID(),
		outputIDMutex:       sync.RWMutex{},
		balances:            NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}),
		aliasAddress:        *randAliasAddress(),
		stateAddress:        governingAliasStateWallet.address,
		stateIndex:          10,
		stateData:           []byte("some data"),
		immutableData:       []byte("some data"),
		isGovernanceUpdate:  false,
		governingAddress:    randAliasAddress(),
		StorableObjectFlags: objectstorage.StorableObjectFlags{},
	}
	aliasStateWallet := genRandomWallet()
	alias := &AliasOutput{
		outputID:            randOutputID(),
		outputIDMutex:       sync.RWMutex{},
		balances:            NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}),
		aliasAddress:        *randAliasAddress(),
		stateAddress:        aliasStateWallet.address,
		stateIndex:          10,
		stateData:           []byte("some data"),
		immutableData:       []byte("some data"),
		isGovernanceUpdate:  false,
		governingAddress:    governingAlias.GetAliasAddress(),
		StorableObjectFlags: objectstorage.StorableObjectFlags{},
	}
	t.Run("CASE: Happy path", func(t *testing.T) {
		// unlocked for gov transition
		nextAlias := alias.NewAliasOutputNext(true)
		// we are updating the state address (simulate committer rotation)
		nextAlias.stateAddress = randEd25119Address()
		// unlocked for state transition
		nextGoverningAlias := governingAlias.NewAliasOutputNext(false)

		outputs := Outputs{nextAlias, nextGoverningAlias}
		inputs := Outputs{}
		inputsOfTx := NewInputs(NewUTXOInput(alias.ID()), NewUTXOInput(governingAlias.ID()))
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, inputsOfTx, NewOutputs(outputs...))

		var indexOfAliasInput, indexOfGoverningAliasInput int
		for i, input := range inputsOfTx {
			castedInput := input.(*UTXOInput)
			if castedInput.referencedOutputID == alias.ID() {
				indexOfAliasInput = i
				inputs = append(inputs, alias)
			}
			if castedInput.referencedOutputID == governingAlias.ID() {
				indexOfGoverningAliasInput = i
				inputs = append(inputs, governingAlias)
			}
		}
		unlocks := make(UnlockBlocks, len(inputsOfTx))
		unlocks[indexOfAliasInput] = NewAliasUnlockBlock(uint16(indexOfGoverningAliasInput))
		unlocks[indexOfGoverningAliasInput] = NewSignatureUnlockBlock(governingAliasStateWallet.sign(essence))

		tx := NewTransaction(essence, unlocks)

		ok, err := alias.unlockedGovernanceByAliasIndex(tx, uint16(indexOfGoverningAliasInput), inputs)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("CASE: Self governed alias can't be unlocked by alias reference", func(t *testing.T) {
		dummyAlias := dummyAliasOutput()
		dummyAlias.governingAddress = nil

		ok, err := dummyAlias.unlockedGovernanceByAliasIndex(&Transaction{}, 0, Outputs{})
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("CASE: Governing address is not alias", func(t *testing.T) {
		dummyAlias := dummyAliasOutput()
		dummyAlias.governingAddress = randEd25119Address()

		ok, err := dummyAlias.unlockedGovernanceByAliasIndex(&Transaction{}, 0, Outputs{})
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("CASE: Invalid referenced index", func(t *testing.T) {
		dummyAlias := dummyAliasOutput()

		ok, err := dummyAlias.unlockedGovernanceByAliasIndex(&Transaction{}, 1, Outputs{})
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("CASE: Referenced output is not an alias", func(t *testing.T) {
		dummyAlias := dummyAliasOutput()

		ok, err := dummyAlias.unlockedGovernanceByAliasIndex(&Transaction{}, 0, Outputs{NewSigLockedSingleOutput(DustThresholdAliasOutputIOTA, randEd25119Address())})
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("CASE: Referenced output has different alias address", func(t *testing.T) {
		dummyAlias := dummyAliasOutput()
		dummyGoverningAlias := dummyAliasOutput()

		ok, err := dummyAlias.unlockedGovernanceByAliasIndex(&Transaction{}, 0, Outputs{dummyGoverningAlias})
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, ok)
	})
}

func TestAliasOutput_UnlockValid(t *testing.T) {
	w := genRandomWallet()
	governingWallet := genRandomWallet()
	alias := &AliasOutput{
		outputID:            randOutputID(),
		outputIDMutex:       sync.RWMutex{},
		balances:            NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}),
		aliasAddress:        *randAliasAddress(),
		stateAddress:        w.address,
		stateIndex:          10,
		stateData:           []byte("some data"),
		immutableData:       []byte("some immutable data"),
		isGovernanceUpdate:  false,
		governingAddress:    governingWallet.address,
		StorableObjectFlags: objectstorage.StorableObjectFlags{},
	}

	t.Run("CASE: Alias unlocked by signature", func(t *testing.T) {
		chained := alias.NewAliasOutputNext(false)
		outputs := Outputs{chained}
		inputs := Outputs{alias}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(alias.ID())), NewOutputs(outputs...))
		unlockBlocks := w.unlockBlocks(essence)
		// w.unlockBlocks puts a signature unlock block for all inputs
		tx := NewTransaction(essence, unlockBlocks)

		valid, err := alias.UnlockValid(tx, unlockBlocks[0], inputs)
		assert.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("CASE: Alias can't be unlocked by invalid signature", func(t *testing.T) {
		chained := alias.NewAliasOutputNext(false)
		clonedAlias := alias.clone()
		clonedAlias.stateAddress = randEd25119Address()
		outputs := Outputs{chained}
		inputs := Outputs{clonedAlias}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(clonedAlias.ID())), NewOutputs(outputs...))
		// sign with bad signature
		unlockBlocks := w.unlockBlocks(essence)
		tx := NewTransaction(essence, unlockBlocks)

		valid, err := clonedAlias.UnlockValid(tx, unlockBlocks[0], inputs)
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("CASE: Alias output destroyed, no gov update", func(t *testing.T) {
		outputs := Outputs{NewSigLockedSingleOutput(DustThresholdAliasOutputIOTA, randEd25119Address())}
		inputs := Outputs{alias}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(alias.ID())), NewOutputs(outputs...))
		unlockBlocks := w.unlockBlocks(essence)
		// w.unlockBlocks puts a signature unlock block for all inputs
		tx := NewTransaction(essence, unlockBlocks)

		valid, err := alias.UnlockValid(tx, unlockBlocks[0], inputs)
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("CASE: Alias output destroyed, gov update", func(t *testing.T) {
		outputs := Outputs{NewSigLockedSingleOutput(DustThresholdAliasOutputIOTA, randEd25119Address())}
		inputs := Outputs{alias}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(alias.ID())), NewOutputs(outputs...))
		unlockBlocks := governingWallet.unlockBlocks(essence)
		// w.unlockBlocks puts a signature unlock block for all inputs
		tx := NewTransaction(essence, unlockBlocks)

		valid, err := alias.UnlockValid(tx, unlockBlocks[0], inputs)
		assert.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("CASE: Alias output can't be destroyed, gov update", func(t *testing.T) {
		clonedAlias := alias.clone()
		clonedAlias.balances = NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA + 1})
		outputs := Outputs{NewSigLockedSingleOutput(DustThresholdAliasOutputIOTA, randEd25119Address())}
		inputs := Outputs{clonedAlias}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(clonedAlias.ID())), NewOutputs(outputs...))
		unlockBlocks := governingWallet.unlockBlocks(essence)
		// w.unlockBlocks puts a signature unlock block for all inputs
		tx := NewTransaction(essence, unlockBlocks)

		valid, err := clonedAlias.UnlockValid(tx, unlockBlocks[0], inputs)
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("CASE: Duplicated alias output", func(t *testing.T) {
		chained := alias.NewAliasOutputNext(false)
		clonedChained := chained.clone()
		// need to change some bytes not to be considered duplicate already in NewOutputs()
		clonedChained.stateData = []byte("random data")
		outputs := Outputs{chained, clonedChained}
		inputs := Outputs{alias}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(alias.ID())), NewOutputs(outputs...))
		unlockBlocks := w.unlockBlocks(essence)
		// w.unlockBlocks puts a signature unlock block for all inputs
		tx := NewTransaction(essence, unlockBlocks)

		valid, err := alias.UnlockValid(tx, unlockBlocks[0], inputs)
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("CASE: Governance update, sig valid", func(t *testing.T) {
		chained := alias.NewAliasOutputNext(true)
		outputs := Outputs{chained}
		inputs := Outputs{alias}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(alias.ID())), NewOutputs(outputs...))
		unlockBlocks := governingWallet.unlockBlocks(essence)
		// w.unlockBlocks puts a signature unlock block for all inputs
		tx := NewTransaction(essence, unlockBlocks)

		valid, err := alias.UnlockValid(tx, unlockBlocks[0], inputs)
		assert.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("CASE: Governance update, sig invalid", func(t *testing.T) {
		chained := alias.NewAliasOutputNext(true)
		outputs := Outputs{chained}
		inputs := Outputs{alias}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(alias.ID())), NewOutputs(outputs...))
		unlockBlocks := w.unlockBlocks(essence)
		// w.unlockBlocks puts a signature unlock block for all inputs
		tx := NewTransaction(essence, unlockBlocks)

		valid, err := alias.UnlockValid(tx, unlockBlocks[0], inputs)
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("CASE: Governance update, transition invalid", func(t *testing.T) {
		chained := alias.NewAliasOutputNext(true)
		chained.stateData = []byte("this should not be changed")
		outputs := Outputs{chained}
		inputs := Outputs{alias}
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, NewInputs(NewUTXOInput(alias.ID())), NewOutputs(outputs...))
		unlockBlocks := governingWallet.unlockBlocks(essence)
		// w.unlockBlocks puts a signature unlock block for all inputs
		tx := NewTransaction(essence, unlockBlocks)

		valid, err := alias.UnlockValid(tx, unlockBlocks[0], inputs)
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("CASE: Unlocked by other alias", func(t *testing.T) {
		governingAliasStateWallet := genRandomWallet()
		governingAlias := &AliasOutput{
			outputID:            randOutputID(),
			outputIDMutex:       sync.RWMutex{},
			balances:            NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}),
			aliasAddress:        *randAliasAddress(),
			stateAddress:        governingAliasStateWallet.address,
			stateIndex:          10,
			stateData:           []byte("some data"),
			immutableData:       []byte("some data"),
			isGovernanceUpdate:  false,
			governingAddress:    randAliasAddress(),
			StorableObjectFlags: objectstorage.StorableObjectFlags{},
		}
		aliasStateWallet := genRandomWallet()
		governedAlias := &AliasOutput{
			outputID:            randOutputID(),
			outputIDMutex:       sync.RWMutex{},
			balances:            NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}),
			aliasAddress:        *randAliasAddress(),
			stateAddress:        aliasStateWallet.address,
			stateIndex:          10,
			stateData:           []byte("some data"),
			immutableData:       []byte("some data"),
			isGovernanceUpdate:  false,
			governingAddress:    governingAlias.GetAliasAddress(),
			StorableObjectFlags: objectstorage.StorableObjectFlags{},
		}
		// unlocked for gov transition
		nextAlias := governedAlias.NewAliasOutputNext(true)
		// we are updating the state address (simulate committer rotation)
		nextAlias.stateAddress = randEd25119Address()
		// unlocked for state transition
		nextGoverningAlias := governingAlias.NewAliasOutputNext(false)

		outputs := Outputs{nextAlias, nextGoverningAlias}
		inputs := Outputs{}
		inputsOfTx := NewInputs(NewUTXOInput(governedAlias.ID()), NewUTXOInput(governingAlias.ID()))
		essence := NewTransactionEssence(0, time.Time{}, identity.ID{}, identity.ID{}, inputsOfTx, NewOutputs(outputs...))

		var indexOfAliasInput, indexOfGoverningAliasInput int
		for i, input := range inputsOfTx {
			castedInput := input.(*UTXOInput)
			if castedInput.referencedOutputID == governedAlias.ID() {
				indexOfAliasInput = i
				inputs = append(inputs, governedAlias)
			}
			if castedInput.referencedOutputID == governingAlias.ID() {
				indexOfGoverningAliasInput = i
				inputs = append(inputs, governingAlias)
			}
		}
		unlocks := make(UnlockBlocks, len(inputsOfTx))
		unlocks[indexOfAliasInput] = NewAliasUnlockBlock(uint16(indexOfGoverningAliasInput))
		unlocks[indexOfGoverningAliasInput] = NewSignatureUnlockBlock(governingAliasStateWallet.sign(essence))

		tx := NewTransaction(essence, unlocks)

		ok, err := governedAlias.UnlockValid(tx, unlocks[indexOfAliasInput], inputs)
		assert.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("CASE: Unsupported unlock block", func(t *testing.T) {
		ok, err := alias.UnlockValid(&Transaction{}, NewReferenceUnlockBlock(0), Outputs{})
		t.Log(err)
		assert.Error(t, err)
		assert.False(t, ok)
	})
}

func TestAliasOutput_Clone(t *testing.T) {
	out := dummyAliasOutput()
	outBack := out.Clone()
	outBackT, ok := outBack.(*AliasOutput)
	assert.True(t, ok)
	assert.True(t, out != outBackT)
	assert.True(t, out.stateAddress != outBackT.stateAddress)
	assert.True(t, out.governingAddress != outBackT.governingAddress)
	assert.True(t, notSameMemory(out.immutableData, outBackT.immutableData))
	assert.True(t, notSameMemory(out.stateData, outBackT.stateData))
	assert.EqualValues(t, out.Bytes(), outBack.Bytes())
}

// endregion

// region ExtendedLockedOutput Tests

func TestExtendedLockedOutput_Address(t *testing.T) {
	t.Run("CASE: Address is signature backed", func(t *testing.T) {
		addy := randEd25119Address()
		o := &ExtendedLockedOutput{address: addy}
		assert.True(t, o.Address().Equals(addy))
	})

	t.Run("CASE: Address is alias address", func(t *testing.T) {
		addy := randAliasAddress()
		o := &ExtendedLockedOutput{address: addy}
		assert.True(t, o.Address().Equals(addy))
	})
}

func TestExtendedLockedOutput_Balances(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		bal := NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA})
		o := &ExtendedLockedOutput{balances: bal}
		assert.Equal(t, bal.Bytes(), o.Balances().Bytes())
	})
}

func TestExtendedLockedOutput_Bytes(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		o := NewExtendedLockedOutput(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}, randEd25119Address()).
			WithFallbackOptions(randEd25119Address(), time.Now().Add(2*time.Hour)).
			WithTimeLock(time.Now().Add(1 * time.Hour))
		err := o.SetPayload([]byte("some metadata"))
		oBytes := o.Bytes()
		var restored Output
		restored, _, err = OutputFromBytes(oBytes)
		assert.NoError(t, err)
		castedRestored, ok := restored.(*ExtendedLockedOutput)
		assert.True(t, ok)
		assert.Equal(t, o.balances.Bytes(), castedRestored.balances.Bytes())
		assert.True(t, o.address.Equals(castedRestored.address))
		assert.Equal(t, o.id.Bytes(), castedRestored.id.Bytes())
		assert.True(t, o.fallbackDeadline.Equal(castedRestored.fallbackDeadline))
		assert.True(t, o.fallbackAddress.Equals(castedRestored.fallbackAddress))
		assert.True(t, o.timelock.Equal(castedRestored.timelock))
		assert.Equal(t, o.payload, castedRestored.payload)

	})
}

func TestExtendedLockedOutput_Clone(t *testing.T) {
	out := dummyExtendedLockedOutput()
	outBack := out.Clone()
	outBackT, ok := outBack.(*ExtendedLockedOutput)
	assert.True(t, ok)
	assert.True(t, out != outBackT)
	assert.True(t, notSameMemory(out.payload, outBackT.payload))
	assert.True(t, out.address != outBackT.address)
	assert.True(t, out.fallbackAddress != outBackT.fallbackAddress)
	assert.EqualValues(t, out.Bytes(), outBack.Bytes())
}

// endregion

// region test utils

func genRandomWallet() wallet {
	kp := ed25519.GenerateKeyPair()
	return wallet{
		kp,
		NewED25519Address(kp.PublicKey),
	}
}

func notSameMemory(s1, s2 []byte) bool {
	if s1 == nil || s2 == nil {
		return true
	}
	return &s1[cap(s1)-1] != &s2[cap(s2)-1]
}

func dummyAliasOutput(origin ...bool) *AliasOutput {
	orig := false
	if len(origin) > 0 {
		orig = origin[0]
	}
	return &AliasOutput{
		outputID:            randOutputID(),
		outputIDMutex:       sync.RWMutex{},
		balances:            NewColoredBalances(map[Color]uint64{ColorIOTA: DustThresholdAliasOutputIOTA}),
		aliasAddress:        *randAliasAddress(),
		stateAddress:        randEd25119Address(),
		stateIndex:          0,
		stateData:           []byte("initial"),
		immutableData:       []byte("don't touch this"),
		isGovernanceUpdate:  false,
		isOrigin:            orig,
		governingAddress:    randAliasAddress(),
		StorableObjectFlags: objectstorage.StorableObjectFlags{},
	}
}

func dummyExtendedLockedOutput() *ExtendedLockedOutput {
	return &ExtendedLockedOutput{
		id:                  randOutputID(),
		idMutex:             sync.RWMutex{},
		balances:            NewColoredBalances(map[Color]uint64{ColorIOTA: 1}),
		address:             randEd25119Address(),
		fallbackAddress:     randEd25119Address(),
		fallbackDeadline:    time.Unix(1001, 0),
		timelock:            time.Unix(2000, 0),
		payload:             []byte("a payload"),
		StorableObjectFlags: objectstorage.StorableObjectFlags{},
	}
}

func randEd25119Address() *ED25519Address {
	keyPair := ed25519.GenerateKeyPair()
	return NewED25519Address(keyPair.PublicKey)
}

func randAliasAddress() *AliasAddress {
	randOutputIDBytes := make([]byte, 34)
	_, _ = rand.Read(randOutputIDBytes)
	return NewAliasAddress(randOutputIDBytes)
}

func randOutputID() OutputID {
	randOutputIDBytes := make([]byte, 34)
	_, _ = rand.Read(randOutputIDBytes)
	outputID, _, _ := OutputIDFromBytes(randOutputIDBytes)
	return outputID
}

// endregion
