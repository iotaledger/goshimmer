package wallet

import (
	"context"
	"strconv"

	"github.com/capossele/asset-registry/pkg/registryclient"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/serializer/v2/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

const (
	// RegistryHostURL is the host url of the central registry.
	RegistryHostURL = "http://asset-registry.tokenizedassetsdemo.iota.cafe"
)

// AssetRegistry represents a registry for colored coins, that stores the relevant metadata in a dictionary.
type AssetRegistry struct {
	assetRegistryInner `serix:"0"`
}

type assetRegistryInner struct {
	Network string                   `serix:"0,lengthPrefixType=uint32"`
	Assets  map[devnetvm.Color]Asset `serix:"1,lengthPrefixType=uint32"`
	// client communicates with the central registry
	client *registryclient.HTTPClient
}

// NewAssetRegistry is the constructor for the AssetRegistry.
func NewAssetRegistry(network string, registryURL ...string) *AssetRegistry {
	hostURL := RegistryHostURL
	if len(registryURL) > 0 {
		hostURL = registryURL[0]
	}
	client := registryclient.NewHTTPClient(resty.New().SetHostURL(hostURL))
	return &AssetRegistry{
		assetRegistryInner{
			Assets:  make(map[devnetvm.Color]Asset),
			client:  client,
			Network: network,
		},
	}
}

// ParseAssetRegistry is a utility function that can be used to parse a marshaled version of the registry.
func ParseAssetRegistry(marshalUtil *marshalutil.MarshalUtil) (assetRegistry *AssetRegistry, consumedBytes int, err error) {
	assetRegistry = new(AssetRegistry)
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), marshalUtil.Bytes()[marshalUtil.ReadOffset():], &assetRegistry, serix.WithValidation())
	if err != nil {
		err = errors.Wrap(err, "failed to parse AssetRegistry")
		return
	}
	marshalUtil.ReadSeek(marshalUtil.ReadOffset() + consumedBytes)
	for color, asset := range assetRegistry.Assets {
		asset.Color = color
	}
	return
}

// SetRegistryURL sets the url of the registry api server.
func (a *AssetRegistry) SetRegistryURL(url string) {
	a.client = registryclient.NewHTTPClient(resty.New().SetHostURL(url))
}

// Network returns the current network the asset registry connects to.
func (a *AssetRegistry) Network() string {
	return a.assetRegistryInner.Network
}

// LoadAsset returns an asset either from local or from central registry.
func (a *AssetRegistry) LoadAsset(id devnetvm.Color) (*Asset, error) {
	_, ok := a.Assets[id]
	if !ok {
		success := a.updateLocalFromCentral(id)
		if !success {
			return nil, errors.Errorf("no asset found with assetID (color) %s", id.Base58())
		}
	}
	asset := a.Assets[id]
	return &asset, nil
}

// RegisterAsset registers an asset in the registry, so we can look up names and symbol of colored coins.
func (a *AssetRegistry) RegisterAsset(color devnetvm.Color, asset Asset) error {
	a.Assets[color] = asset
	return a.client.SaveAsset(context.TODO(), a.Network(), asset.ToRegistry())
}

// Name returns the name of the given asset.
func (a *AssetRegistry) Name(color devnetvm.Color) string {
	// check in local registry
	if asset, assetExists := a.Assets[color]; assetExists {
		return asset.Name
	}

	if color == devnetvm.ColorIOTA {
		return "IOTA"
	}
	// not in local
	// fetch from central, update local
	if a.updateLocalFromCentral(color) {
		return a.Assets[color].Name
	}
	// fallback if we fetch was not successful, just use the color as name
	return color.String()
}

// Symbol return the symbol of the token.
func (a *AssetRegistry) Symbol(color devnetvm.Color) string {
	if asset, assetExists := a.Assets[color]; assetExists {
		return asset.Symbol
	}

	if color == devnetvm.ColorIOTA {
		return "I"
	}

	// not in local
	// fetch from central, update local
	if a.updateLocalFromCentral(color) {
		return a.Assets[color].Symbol
	}

	return "cI"
}

// Supply returns the initial supply of the token.
func (a *AssetRegistry) Supply(color devnetvm.Color) string {
	if asset, assetExists := a.Assets[color]; assetExists {
		return strconv.FormatUint(asset.Supply, 10)
	}

	if color == devnetvm.ColorIOTA {
		return ""
	}

	// not in local
	// fetch from central, update local
	if a.updateLocalFromCentral(color) {
		return strconv.FormatUint(a.Assets[color].Supply, 10)
	}

	return "unknown"
}

// TransactionID returns the ID of the transaction that created the token.
func (a *AssetRegistry) TransactionID(color devnetvm.Color) string {
	if asset, assetExists := a.Assets[color]; assetExists {
		return asset.TransactionID.Base58()
	}

	if color == devnetvm.ColorIOTA {
		return ""
	}

	// not in local
	// fetch from central, update local
	if a.updateLocalFromCentral(color) {
		return a.Assets[color].TransactionID.Base58()
	}

	return "unknown"
}

// Precision returns the amount of decimal places that this token uses.
func (a *AssetRegistry) Precision(color devnetvm.Color) int {
	if asset, assetExists := a.Assets[color]; assetExists {
		return int(asset.Precision)
	}

	return 0
}

// Bytes marshal the registry into a sequence of bytes.
func (a *AssetRegistry) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), a, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

func (a *AssetRegistry) updateLocalFromCentral(color devnetvm.Color) (success bool) {
	loadedAsset, err := a.client.LoadAsset(context.TODO(), a.Network(), color.Base58())
	if err == nil {
		// save it locally
		var walletAsset *Asset
		walletAsset, err = AssetFromRegistryEntry(loadedAsset)
		if err == nil {
			a.Assets[walletAsset.Color] = *walletAsset
			return true
		}
	}
	return false
}
