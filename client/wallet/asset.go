package wallet

import (
	"github.com/capossele/asset-registry/pkg/registry"
	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// Asset represents a container for all the information regarding a colored coin.
type Asset struct {
	// Color contains the identifier of this asset
	Color ledgerstate.Color

	// Name of the asset
	Name string `serix:"1,lengthPrefixType=uint32"`

	// currency symbol of the asset (optional)
	Symbol string `serix:"2,lengthPrefixType=uint32"`

	// Precision defines how many decimal places are shown when showing this asset in wallets
	Precision uint32 `serix:"3"`

	// Supply is the amount of tokens that we want to create
	Supply uint64 `serix:"4"`

	// TransactionID that created the asset
	TransactionID ledgerstate.TransactionID `serix:"5"`
}

// ToRegistry creates a ergistry asset from a wallet asset.
func (a *Asset) ToRegistry() *registry.Asset {
	return &registry.Asset{
		ID:            a.Color.Base58(),
		Name:          a.Name,
		Symbol:        a.Symbol,
		Supply:        a.Supply,
		TransactionID: a.TransactionID.Base58(),
	}
}

// AssetFromRegistryEntry creates a wallet asset from a registry asset.
func AssetFromRegistryEntry(regAsset *registry.Asset) (*Asset, error) {
	color, err := ledgerstate.ColorFromBase58EncodedString(regAsset.ID)
	if err != nil {
		return nil, errors.Errorf("failed to parse color(ID) of asset from registry response: %w, err")
	}
	var txID ledgerstate.TransactionID
	txID, err = ledgerstate.TransactionIDFromBase58(regAsset.TransactionID)
	if err != nil {
		return nil, errors.Errorf("failed to parse TransactionID of asset from registry response: %w, err")
	}
	return &Asset{
		Color:         color,
		Name:          regAsset.Name,
		Symbol:        regAsset.Symbol,
		Supply:        regAsset.Supply,
		TransactionID: txID,
	}, nil
}
