package wallet

import (
	"context"
	"strconv"

	"github.com/capossele/asset-registry/pkg/registryclient"
	"github.com/capossele/asset-registry/pkg/registryservice"
	"github.com/cockroachdb/errors"
	"github.com/go-resty/resty/v2"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/typeutils"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

const (
	// RegistryHostURL is the host url of the central registry.
	RegistryHostURL = "http://asset-registry.tokenizedassetsdemo.iota.cafe"
)

// AssetRegistry represents a registry for colored coins, that stores the relevant metadata in a dictionary.
type AssetRegistry struct {
	assets map[ledgerstate.Color]Asset
	// client communicates with the central registry
	client  *registryclient.HTTPClient
	network string
}

// NewAssetRegistry is the constructor for the AssetRegistry.
func NewAssetRegistry(network string, registryURL ...string) *AssetRegistry {
	hostURL := RegistryHostURL
	if len(registryURL) > 0 {
		hostURL = registryURL[0]
	}
	client := registryclient.NewHTTPClient(resty.New().SetHostURL(hostURL))
	return &AssetRegistry{
		make(map[ledgerstate.Color]Asset),
		client,
		network,
	}
}

// ParseAssetRegistry is a utility function that can be used to parse a marshaled version of the registry.
func ParseAssetRegistry(marshalUtil *marshalutil.MarshalUtil) (assetRegistry *AssetRegistry, consumedBytes int, err error) {
	startingOffset := marshalUtil.ReadOffset()

	networkLength, parseErr := marshalUtil.ReadUint32()
	if parseErr != nil {
		err = parseErr
		return
	}
	networkBytes, parseErr := marshalUtil.ReadBytes(int(networkLength))
	if parseErr != nil {
		err = parseErr
		return
	}
	network := typeutils.BytesToString(networkBytes)
	if !registryservice.Networks[network] {
		err = errors.Errorf("unsupported asset registry network: %s", network)
		return
	}
	assetRegistry = NewAssetRegistry(network)

	assetCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return
	}

	for i := uint64(0); i < assetCount; i++ {
		asset := Asset{}

		colorBytes, parseErr := marshalUtil.ReadBytes(ledgerstate.ColorLength)
		if parseErr != nil {
			err = parseErr

			return
		}
		color, _, parseErr := ledgerstate.ColorFromBytes(colorBytes)
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

		supply, parseErr := marshalUtil.ReadUint64()
		if parseErr != nil {
			err = parseErr

			return
		}

		txID, parseErr := ledgerstate.TransactionIDFromMarshalUtil(marshalUtil)
		if parseErr != nil {
			err = parseErr

			return
		}

		asset.Color = color
		asset.Name = typeutils.BytesToString(nameBytes)
		asset.Symbol = typeutils.BytesToString(symbolBytes)
		asset.Precision = int(precision)
		asset.Supply = supply
		asset.TransactionID = txID

		assetRegistry.assets[color] = asset
	}

	consumedBytes = marshalUtil.ReadOffset() - startingOffset

	return
}

// SetRegistryURL sets the url of the registry api server.
func (a *AssetRegistry) SetRegistryURL(url string) {
	a.client = registryclient.NewHTTPClient(resty.New().SetHostURL(url))
}

// Network returns the current network the asset registry connects to.
func (a *AssetRegistry) Network() string {
	return a.network
}

// LoadAsset returns an asset either from local or from central registry.
func (a *AssetRegistry) LoadAsset(id ledgerstate.Color) (*Asset, error) {
	_, ok := a.assets[id]
	if !ok {
		success := a.updateLocalFromCentral(id)
		if !success {
			return nil, errors.Errorf("no asset found with assetID (color) %s", id.Base58())
		}
	}
	asset := a.assets[id]
	return &asset, nil
}

// RegisterAsset registers an asset in the registry, so we can look up names and symbol of colored coins.
func (a *AssetRegistry) RegisterAsset(color ledgerstate.Color, asset Asset) error {
	a.assets[color] = asset
	return a.client.SaveAsset(context.TODO(), a.network, asset.ToRegistry())
}

// Name returns the name of the given asset.
func (a *AssetRegistry) Name(color ledgerstate.Color) string {
	// check in local registry
	if asset, assetExists := a.assets[color]; assetExists {
		return asset.Name
	}

	if color == ledgerstate.ColorIOTA {
		return "IOTA"
	}
	// not in local
	// fetch from central, update local
	if a.updateLocalFromCentral(color) {
		return a.assets[color].Name
	}
	// fallback if we fetch was not successful, just use the color as name
	return color.String()
}

// Symbol return the symbol of the token.
func (a *AssetRegistry) Symbol(color ledgerstate.Color) string {
	if asset, assetExists := a.assets[color]; assetExists {
		return asset.Symbol
	}

	if color == ledgerstate.ColorIOTA {
		return "I"
	}

	// not in local
	// fetch from central, update local
	if a.updateLocalFromCentral(color) {
		return a.assets[color].Symbol
	}

	return "cI"
}

// Supply returns the initial supply of the token.
func (a *AssetRegistry) Supply(color ledgerstate.Color) string {
	if asset, assetExists := a.assets[color]; assetExists {
		return strconv.FormatUint(asset.Supply, 10)
	}

	if color == ledgerstate.ColorIOTA {
		return ""
	}

	// not in local
	// fetch from central, update local
	if a.updateLocalFromCentral(color) {
		return strconv.FormatUint(a.assets[color].Supply, 10)
	}

	return "unknown"
}

// TransactionID returns the ID of the transaction that created the token.
func (a *AssetRegistry) TransactionID(color ledgerstate.Color) string {
	if asset, assetExists := a.assets[color]; assetExists {
		return asset.TransactionID.Base58()
	}

	if color == ledgerstate.ColorIOTA {
		return ""
	}

	// not in local
	// fetch from central, update local
	if a.updateLocalFromCentral(color) {
		return a.assets[color].TransactionID.Base58()
	}

	return "unknown"
}

// Precision returns the amount of decimal places that this token uses.
func (a *AssetRegistry) Precision(color ledgerstate.Color) int {
	if asset, assetExists := a.assets[color]; assetExists {
		return asset.Precision
	}

	return 0
}

// Bytes marshal the registry into a sequence of bytes.
func (a *AssetRegistry) Bytes() []byte {
	marshalUtil := marshalutil.New()

	networkBytes := typeutils.StringToBytes(a.network)
	marshalUtil.WriteUint32(uint32(len(networkBytes)))
	marshalUtil.WriteBytes(networkBytes)

	assetCount := len(a.assets)
	marshalUtil.WriteUint64(uint64(assetCount))

	for color, asset := range a.assets {
		marshalUtil.WriteBytes(color.Bytes())

		nameBytes := typeutils.StringToBytes(asset.Name)
		marshalUtil.WriteUint32(uint32(len(nameBytes)))
		marshalUtil.WriteBytes(nameBytes)

		symbolBytes := typeutils.StringToBytes(asset.Symbol)
		marshalUtil.WriteUint32(uint32(len(symbolBytes)))
		marshalUtil.WriteBytes(symbolBytes)

		marshalUtil.WriteUint32(uint32(asset.Precision))
		marshalUtil.WriteUint64(asset.Supply)
		marshalUtil.WriteBytes(asset.TransactionID.Bytes())
	}

	return marshalUtil.Bytes()
}

func (a *AssetRegistry) updateLocalFromCentral(color ledgerstate.Color) (success bool) {
	loadedAsset, err := a.client.LoadAsset(context.TODO(), a.network, color.Base58())
	if err == nil {
		// save it locally
		var walletAsset *Asset
		walletAsset, err = AssetFromRegistryEntry(loadedAsset)
		if err == nil {
			a.assets[walletAsset.Color] = *walletAsset
			return true
		}
	}
	return false
}
