package ledgerstate

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"
)

func TestAliasOutput_NewAliasOutputMint(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		stateAddy := randEd25119Address()
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: 100}, stateAddy)
		assert.NoError(t, err)
		iotaBal, ok := alias.Balances().Get(ColorIOTA)
		assert.True(t, ok)
		assert.Equal(t, uint64(100), iotaBal)
		assert.True(t, alias.GetStateAddress().Equals(stateAddy))
		assert.Nil(t, alias.GetImmutableData())
	})

	t.Run("CASE: Happy path with immutable data", func(t *testing.T) {
		stateAddy := randEd25119Address()
		data := []byte("dummy")
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: 100}, stateAddy, data)
		assert.NoError(t, err)
		iotaBal, ok := alias.Balances().Get(ColorIOTA)
		assert.True(t, ok)
		assert.Equal(t, uint64(100), iotaBal)
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
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: 100}, stateAddy, data)
		assert.Error(t, err)
		assert.Nil(t, alias)
	})

	t.Run("CASE: Non existent state address", func(t *testing.T) {
		data := []byte("dummy")
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: 100}, nil, data)
		assert.Error(t, err)
		assert.Nil(t, alias)
	})

	t.Run("CASE: Too big state data", func(t *testing.T) {
		stateAddy := randAliasAddress()
		data := make([]byte, MaxOutputPayloadSize+1)
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: 100}, stateAddy, data)
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
		originAlias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: 100}, randEd25119Address())
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

func TestAliasOutput_GetAliasAddress(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		zeroAddy := &AliasAddress{}
		alias.aliasAddress = *zeroAddy
		calcAliasAddress := alias.GetAliasAddress()
		assert.True(t, calcAliasAddress.Equals(NewAliasAddress(alias.ID().Bytes())))
	})
}

func TestAliasOutput_Address(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		zeroAddy := &AliasAddress{}
		alias.aliasAddress = *zeroAddy
		assert.True(t, alias.Address().Equals(NewAliasAddress(alias.ID().Bytes())))
	})
}

func TestAliasOutput_Balances(t *testing.T) {
	t.Run("CASE: Happy path", func(t *testing.T) {
		alias := dummyAliasOutput()
		assert.Equal(t, NewColoredBalances(map[Color]uint64{ColorIOTA: 100}).Bytes(), alias.Balances().Bytes())
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
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: 100}, randEd25119Address())
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
		alias, err := NewAliasOutputMint(map[Color]uint64{ColorIOTA: 100}, randEd25119Address())
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
			ColorIOTA: 100,
			ColorMint: 500,
		})
		updated := alias.UpdateMintingColor()
		balance, ok := updated.Balances().Get(blake2b.Sum256(alias.ID().Bytes()))
		assert.True(t, ok)
		assert.Equal(t, uint64(500), balance)
	})

	t.Run("CASE: No mint", func(t *testing.T) {
		alias := dummyAliasOutput()
		alias.balances = NewColoredBalances(map[Color]uint64{
			ColorIOTA: 100,
		})
		updated := alias.UpdateMintingColor()
		balance, ok := updated.Balances().Get(blake2b.Sum256(alias.ID().Bytes()))
		assert.False(t, ok)
		balance, ok = updated.Balances().Get(ColorIOTA)
		assert.True(t, ok)
		assert.Equal(t, uint64(100), balance)
	})
}

func TestAliasOutput_UnlockValid(t *testing.T) {
	// TODO: to be continued
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

func notSameMemory(s1, s2 []byte) bool {
	if s1 == nil || s2 == nil {
		return true
	}
	return &s1[cap(s1)-1] != &s2[cap(s2)-1]
}

func dummyAliasOutput() *AliasOutput {
	return &AliasOutput{
		outputID:            randOutputID(),
		outputIDMutex:       sync.RWMutex{},
		balances:            NewColoredBalances(map[Color]uint64{ColorIOTA: 100}),
		aliasAddress:        *randAliasAddress(),
		stateAddress:        randEd25119Address(),
		stateIndex:          0,
		stateData:           []byte("initial"),
		immutableData:       []byte("don't touch this"),
		isGovernanceUpdate:  false,
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
