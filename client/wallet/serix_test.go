package wallet

import (
	"testing"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func TestSerixAssetRegistry(t *testing.T) {
	obj := &AssetRegistry{
		assetRegistryInner{
			Assets:  make(map[ledgerstate.Color]Asset),
			client:  nil,
			Network: "testnetwork",
		},
	}
	obj.assetRegistryInner.Assets[ledgerstate.ColorIOTA] = Asset{
		Color:         ledgerstate.ColorIOTA,
		Name:          "iota",
		Symbol:        "das",
		Precision:     23,
		Supply:        111,
		TransactionID: ledgerstate.TransactionID{},
	}
	obj.assetRegistryInner.Assets[ledgerstate.ColorMint] = Asset{
		Color:         ledgerstate.ColorMint,
		Name:          "iot22a",
		Symbol:        "das",
		Precision:     23,
		Supply:        111,
		TransactionID: ledgerstate.TransactionID{},
	}
	assert.Equal(t, obj.BytesOld(), obj.Bytes())
	objRestored, _, err := ParseAssetRegistry(marshalutil.New(obj.Bytes()))
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), objRestored.Bytes())
}
