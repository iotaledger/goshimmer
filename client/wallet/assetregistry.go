package wallet

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/typeutils"
)

// AssetRegistry represents a registry for colored coins, that stores the relevant metadata in a dictionary.
type AssetRegistry struct {
	assets map[balance.Color]Asset
}

// NewAssetRegistry is the constructor for the AssetRegistry.
func NewAssetRegistry() *AssetRegistry {
	return &AssetRegistry{
		make(map[balance.Color]Asset),
	}
}

// ParseAssetRegistry is a utility function that can be used to parse a marshaled version of the registry.
func ParseAssetRegistry(marshalUtil *marshalutil.MarshalUtil) (assetRegistry *AssetRegistry, consumedBytes int, err error) {
	assetRegistry = &AssetRegistry{
		assets: make(map[balance.Color]Asset),
	}

	startingOffset := marshalUtil.ReadOffset()

	assetCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return
	}

	for i := uint64(0); i < assetCount; i++ {
		asset := Asset{}

		colorBytes, parseErr := marshalUtil.ReadBytes(balance.ColorLength)
		if parseErr != nil {
			err = parseErr

			return
		}
		color, _, parseErr := balance.ColorFromBytes(colorBytes)
		if parseErr != nil {
			err = parseErr

			return
		}

		nameLength, parseErr := marshalUtil.ReadUint32()
		if parseErr != nil {
			err = parseErr

			return
		}
		nameBytes, parseErr := marshalUtil.ReadBytes(int(nameLength))
		if parseErr != nil {
			err = parseErr

			return
		}

		symbolLength, parseErr := marshalUtil.ReadUint32()
		if parseErr != nil {
			err = parseErr

			return
		}
		symbolBytes, parseErr := marshalUtil.ReadBytes(int(symbolLength))
		if parseErr != nil {
			err = parseErr

			return
		}

		precision, parseErr := marshalUtil.ReadUint32()
		if parseErr != nil {
			err = parseErr

			return
		}

		asset.Color = color
		asset.Name = typeutils.BytesToString(nameBytes)
		asset.Symbol = typeutils.BytesToString(symbolBytes)
		asset.Precision = int(precision)

		assetRegistry.assets[color] = asset
	}

	consumedBytes = marshalUtil.ReadOffset() - startingOffset

	return
}

// RegisterAsset registers an asset in the registry, so we can look up names and symbol of colored coins.
func (assetRegistry *AssetRegistry) RegisterAsset(color balance.Color, asset Asset) {
	assetRegistry.assets[color] = asset
}

// Name returns the name of the given asset.
func (assetRegistry *AssetRegistry) Name(color balance.Color) string {
	if asset, assetExists := assetRegistry.assets[color]; assetExists {
		return asset.Name
	}

	if color == balance.ColorIOTA {
		return "IOTA"
	}

	return color.String()
}

// Symbol return the symbol of the token.
func (assetRegistry *AssetRegistry) Symbol(color balance.Color) string {
	if asset, assetExists := assetRegistry.assets[color]; assetExists {
		return asset.Symbol
	}

	if color == balance.ColorIOTA {
		return "I"
	}

	return "cI"
}

// Precision returns the amount of decimal places that this token uses.
func (assetRegistry *AssetRegistry) Precision(color balance.Color) int {
	if asset, assetExists := assetRegistry.assets[color]; assetExists {
		return asset.Precision
	}

	return 0
}

// Bytes marshal the registry into a sequence of bytes.
func (assetRegistry *AssetRegistry) Bytes() []byte {
	marshalUtil := marshalutil.New()

	assetCount := len(assetRegistry.assets)
	marshalUtil.WriteUint64(uint64(assetCount))

	for color, asset := range assetRegistry.assets {
		marshalUtil.WriteBytes(color.Bytes())

		nameBytes := typeutils.StringToBytes(asset.Name)
		marshalUtil.WriteUint32(uint32(len(nameBytes)))
		marshalUtil.WriteBytes(nameBytes)

		symbolBytes := typeutils.StringToBytes(asset.Symbol)
		marshalUtil.WriteUint32(uint32(len(symbolBytes)))
		marshalUtil.WriteBytes(symbolBytes)

		marshalUtil.WriteUint32(uint32(asset.Precision))
	}

	return marshalUtil.Bytes()
}
