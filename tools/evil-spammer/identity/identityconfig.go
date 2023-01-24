package identity

import (
	"encoding/json"
	"fmt"
	"os"
)

// Params matches alias names and corresponding private key strings in Base58 encoding.
type Params struct {
	Network map[string]Identities `json:"network"`
}

type Identities map[string]string

var Config Params

var identityConfigJSON = `{
  "network": [
    "docker": [
	  "peer_master": "8q491c3YWjbPwLmF2WD95YmCgh61j2kenCKHfGfByoWi",
	  "peer_master2": "4ata8GcTRMJ5sSv2jQJWmSYYTHct748p3tXmCFYm7wjA",
	  "faucet": "3YX6e7AL28hHihZewKdq6CMkEYVsTJBLgRiprUNiNq5E"
    ]
  ]
}`

func LoadIdentity(networkName, alias string) string {
	if identities, ok := Config.Network[networkName]; ok {
		if privateKey, exists := identities[alias]; exists {
			return privateKey
		}
	}
	return ""
}

func LoadConfig() Params {
	// open config file
	fname := "keys-config.json"
	file, err := os.Open(fname)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
		if err = os.WriteFile(fname, []byte(identityConfigJSON), 0o644); err != nil {
			panic(err)
		}
		if file, err = os.Open(fname); err != nil {
			panic(err)
		}
	}
	defer file.Close()
	// decode config file
	if err = json.NewDecoder(file).Decode(&Config); err != nil {
		panic(err)
	}
	fmt.Println("Loaded config:", Config)
	return Config
}
