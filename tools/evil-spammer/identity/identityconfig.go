package identity

import (
	"encoding/json"
	"fmt"
	"os"
)

type Params map[string]Identities
type Identities map[string]privKeyURLPair
type privKeyURLPair [2]string

var Config = Params{}

var identityConfigJSON = `{
  "docker": {
    "peer_master": [
      "8q491c3YWjbPwLmF2WD95YmCgh61j2kenCKHfGfByoWi",
      "http://localhost:8080"
    ],
    "peer_master2": [
      "4ata8GcTRMJ5sSv2jQJWmSYYTHct748p3tXmCFYm7wjA",
      "http://localhost:8070"
    ],
    "faucet": [
      "3YX6e7AL28hHihZewKdq6CMkEYVsTJBLgRiprUNiNq5E",
      "http://localhost:8090"
    ]
  }
}`

func LoadIdentity(networkName, alias string) (privKey, url string) {
	fmt.Println("Loading identity", alias, "for network", networkName)
	if networkIdentities, exists := Config[networkName]; exists {
		if keyURLPair, ok := networkIdentities[alias]; ok {
			privateKey := keyURLPair[0]
			urlAPI := keyURLPair[1]
			fmt.Println("Loaded identity", alias, "API url:", url, "for network", networkName, "...DONE")
			return privateKey, urlAPI
		}
		return "", ""
	}
	return "", ""
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
