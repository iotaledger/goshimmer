package config

import (
	"reflect"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/plugins/activity"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/blocklayer"
	"github.com/iotaledger/goshimmer/plugins/dagsvisualizer"
	"github.com/iotaledger/goshimmer/plugins/dashboard"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/p2p"
	"github.com/iotaledger/goshimmer/plugins/pow"
	"github.com/iotaledger/goshimmer/plugins/profiling"
	"github.com/iotaledger/goshimmer/plugins/prometheus"
	"github.com/iotaledger/goshimmer/plugins/webapi"
)

// GoShimmer defines the config of a GoShimmer node.
type GoShimmer struct {
	// Name specifies the GoShimmer instance.
	Name string
	// Image specifies the docker image for the instance
	Image string
	// DisabledPlugins specifies the plugins that are disabled with a config.
	DisabledPlugins []string
	// Seed specifies identity.
	Seed []byte
	// Whether to use the same seed for the node's wallet.
	UseNodeSeedAsWalletSeed bool

	// Network specifies network-level configurations
	Network

	// individual plugin configurations
	Database
	P2P
	Gossip
	POW
	WebAPI
	AutoPeering
	BlockLayer
	Faucet
	Mana
	Consensus
	Activity
	Prometheus
	Profiling
	Dashboard
	Dagsvisualizer
	Notarization
	RateSetter
}

// NewGoShimmer creates a GoShimmer config initialized with default values.
func NewGoShimmer() (config GoShimmer) {
	config = GoShimmer{}
	fillStructFromDefaultTag(reflect.ValueOf(&config).Elem())
	return
}

type Network struct {
	Enabled bool
}

// Database defines the parameters of the database plugin.
type Database struct {
	Enabled bool

	database.ParametersDefinition
}

// P2P defines the parameters of the gossip plugin.
type P2P struct {
	Enabled bool

	p2p.ParametersDefinition
}

// Gossip defines the parameters of the gossip plugin.
type Gossip struct {
	Enabled bool

	gossip.ParametersDefinition
}

// POW defines the parameters of the PoW plugin.
type POW struct {
	Enabled bool

	pow.ParametersDefinition
}

// WebAPI defines the parameters of the Web API plugin.
type WebAPI struct {
	Enabled bool

	webapi.ParametersDefinition
}

// AutoPeering defines the parameters of the autopeering plugin.
type AutoPeering struct {
	Enabled bool

	autopeering.ParametersDefinition
	discovery.ParametersDefinitionDiscovery
}

// Faucet defines the parameters of the faucet plugin.
type Faucet struct {
	Enabled bool

	faucet.ParametersDefinition
}

// Mana defines the parameters of the Mana plugin.
type Mana struct {
	Enabled bool

	blocklayer.ManaParametersDefinition
}

// BlockLayer defines the parameters used by the block layer.
type BlockLayer struct {
	Enabled bool

	blocklayer.ParametersDefinition
}

// Consensus defines the parameters of the consensus plugin.
type Consensus struct {
	Enabled bool
}

// Activity defines the parameters of the activity plugin.
type Activity struct {
	Enabled bool

	activity.ParametersDefinition
}

// RateSetter defines the parameters of the RateSetter plugin.
type RateSetter struct {
	Enabled bool

	blocklayer.RateSetterParametersDefinition
}

// Prometheus defines the parameters of the Prometheus plugin.
type Prometheus struct {
	Enabled bool

	prometheus.ParametersDefinition
}

// Profiling defines the parameters of the Profiling plugin.
type Profiling struct {
	Enabled bool

	profiling.ParametersDefinition
}

// Dashboard defines the parameters of the Dashboard plugin.
type Dashboard struct {
	Enabled bool

	dashboard.ParametersDefinition
}

// Dagsvisualizer defines the parameters of the Dag Visualizer plugin.
type Dagsvisualizer struct {
	Enabled bool

	dagsvisualizer.ParametersDefinition
}

// Notarization defines the parameters of the Notarization plugin.
type Notarization struct {
	Enabled bool

	blocklayer.NotarizationParametersDefinition
}

// CreateIdentity returns an identity based on the config.
// If a Seed is specified, it is used to derive the identity. Otherwise a new key pair is generated and Seed set accordingly.
func (s *GoShimmer) CreateIdentity() (*identity.Identity, error) {
	if s.Seed != nil {
		publicKey := ed25519.PrivateKeyFromSeed(s.Seed).Public()
		return identity.New(publicKey), nil
	}

	publicKey, privateKey, err := ed25519.GenerateKey()
	if err != nil {
		return nil, err
	}
	s.Seed = privateKey.Seed().Bytes()
	return identity.New(publicKey), nil
}
