package config

import (
	"fmt"
	"reflect"
	"time"

	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
)

// GoShimmer defines the config of a GoShimmer node.
type GoShimmer struct {
	// Name specifies the GoShimmer instance.
	Name string
	// DisabledPlugins specifies the plugins that are disabled with a config.
	DisabledPlugins []string
	// Seed specifies identity.
	Seed []byte

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
}

func NewGoShimmer() (config *GoShimmer) {
	config = &GoShimmer{}
	fillStructFromDefaultTag(config)

	fmt.Println("Gossip config:", config.Gossip.TipsBroadcaster.Enable, config.Gossip.TipsBroadcaster.Interval)

	return
}

// fillStructFromDefaultTag recursively explores the given struct pointer and sets values of fields to the `default` as
// specified in the struct's tags.
func fillStructFromDefaultTag(s interface{}) {
	val := reflect.ValueOf(s).Elem()
	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)

		if valueField.Kind() == reflect.Struct {
			fillStructFromDefaultTag(valueField.Addr().Interface())
			continue
		}

		tagDefaultValue, exists := typeField.Tag.Lookup("default")
		if !exists {
			continue
		}

		var err error
		switch defaultValue := valueField.Interface().(type) {
		case bool:
			if _, err = fmt.Sscan(tagDefaultValue, &defaultValue); err != nil {
				panic(err)
			}
			valueField.Set(reflect.ValueOf(defaultValue))
		case time.Duration:
			if defaultValue, err = time.ParseDuration(tagDefaultValue); err != nil {
				panic(err)
			}
			valueField.Set(reflect.ValueOf(defaultValue))
		case int:
			if _, err = fmt.Sscan(tagDefaultValue, &defaultValue); err != nil {
				panic(err)
			}
			valueField.Set(reflect.ValueOf(defaultValue))
		default:
			panic(fmt.Sprintf("type `%s` not implemented", reflect.TypeOf(defaultValue)))
		}
	}
}

// Database defines the parameters of the database plugin.
type Database struct {
	Enabled bool

	ForceCacheTime time.Duration
}

// Gossip defines the parameters of the gossip plugin.
type Gossip struct {
	Enabled bool

	gossip.ParametersType
}

// POW defines the parameters of the PoW plugin.
type POW struct {
	Enabled bool

	Difficulty int
}

// Webapi defines the parameters of the Web API plugin.
type Webapi struct {
	Enabled bool

	BindAddress string
}

// Autopeering defines the parameters of the autopeering plugin.
type Autopeering struct {
	Enabled bool

	Port       int
	EntryNodes []string
}

// Faucet defines the parameters of the faucet plugin.
type Faucet struct {
	Enabled bool

	Seed                 string
	TokensPerRequest     int
	PowDifficulty        int
	PreparedOutputsCount int
}

// Mana defines the parameters of the Mana plugin.
type Mana struct {
	Enabled bool

	AllowedAccessPledge           []string
	AllowedAccessFilterEnabled    bool
	AllowedConsensusPledge        []string
	AllowedConsensusFilterEnabled bool
}

// MessageLayer defines the parameters used by the message layer.
type MessageLayer struct {
	Enabled bool

	Snapshot struct {
		File        string
		GenesisNode string
	}
	FCOB struct {
		QuarantineTime int
	}

	TangleTimeWindow time.Duration
	StartSynced      bool
}

// Consensus defines the parameters of the consensus plugin.
type Consensus struct {
	Enabled bool
}

// FPC defines the parameters used by the FPC consensus.
type FPC struct {
	Enabled bool

	BindAddress             string
	RoundInterval           int
	TotalRoundsFinalization int
}

// Activity defines the parameters of the activity plugin.
type Activity struct {
	Enabled bool

	BroadcastIntervalSec int
}

// DRNG defines the parameters of the DRNG plugin.
type DRNG struct {
	Enabled bool

	Custom struct {
		InstanceId        int
		Threshold         int
		DistributedPubKey string
		CommitteeMembers  []string
	}
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
