package config

import (
	"reflect"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/plugins/activity"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/database"
	"github.com/iotaledger/goshimmer/plugins/drng"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
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

	// Network specifies network-level configurations
	Network

	// individual plugin configurations
	Database
	Gossip
	POW
	Webapi
	Autopeering
	MessageLayer
	Faucet
	Mana
	Consensus
	FPC
	Activity
	DRNG
	Prometheus
	Profiling
}

// NewGoShimmer creates a GoShimmer config initialized with default values.
func NewGoShimmer() (config GoShimmer) {
	config = GoShimmer{}
	fillStructFromDefaultTag(reflect.ValueOf(&config).Elem())
	return
}

type Network struct {
	Enabled bool

	local.ParametersDefinitionNetwork
}

// Database defines the parameters of the database plugin.
type Database struct {
	Enabled bool

	database.ParametersDefinition
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

// Webapi defines the parameters of the Web API plugin.
type Webapi struct {
	Enabled bool

	webapi.ParametersDefinition
}

// Autopeering defines the parameters of the autopeering plugin.
type Autopeering struct {
	Enabled bool

	autopeering.ParametersDefinition
	discovery.ParametersDefinitionDiscovery
	local.ParametersDefinitionLocal
}

// Faucet defines the parameters of the faucet plugin.
type Faucet struct {
	Enabled bool

	faucet.ParametersDefinition
}

// Mana defines the parameters of the Mana plugin.
type Mana struct {
	Enabled bool

	messagelayer.ManaParametersDefinition
}

// MessageLayer defines the parameters used by the message layer.
type MessageLayer struct {
	Enabled bool

	messagelayer.ParametersDefinition
}

// Consensus defines the parameters of the consensus plugin.
type Consensus struct {
	Enabled bool
}

// FPC defines the parameters used by the FPC consensus.
type FPC struct {
	Enabled bool

	messagelayer.FPCParametersDefinition
}

// Activity defines the parameters of the activity plugin.
type Activity struct {
	Enabled bool

	activity.ParametersDefinition
}

// DRNG defines the parameters of the DRNG plugin.
type DRNG struct {
	Enabled bool

	drng.ParametersDefinition
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
