package identity

import (
	"encoding/json"
	"fmt"
	"os"
)

type Identities map[string]string
type Params map[string]Identities

var Config = Params{}

var identityConfigJSON = `{
	"docker": {
	  "peer_master": "8q491c3YWjbPwLmF2WD95YmCgh61j2kenCKHfGfByoWi",
	  "peer_master2": "4ata8GcTRMJ5sSv2jQJWmSYYTHct748p3tXmCFYm7wjA",
	  "faucet": "3YX6e7AL28hHihZewKdq6CMkEYVsTJBLgRiprUNiNq5E"
	}
}`

func LoadIdentity(networkName, alias string) string {
	fmt.Println("Loading identity", alias, "for network", networkName)
	if networkIdentities, exists := Config[networkName]; exists {
		if privateKey, ok := networkIdentities[alias]; ok {
			return privateKey
		}
		return ""
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
		if err = os.WriteFile(fname, []byte(identityConfigJSON), 0o600); err != nil {
			panic(err)
		}
	}
	defer file.Close()
	// decode config file
	fbytes, err := os.ReadFile(fname)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(fbytes, &Config); err != nil {
		panic(err)
	}
	return Config
}
