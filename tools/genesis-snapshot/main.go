package main

import (
	"fmt"
	"log"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/tools/genesis-snapshot/snapshotcreator"

	"github.com/mr-tron/base58"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	cfgGenesisTokenAmount   = "token-amount"
	cfgPledgeTokenAmount    = "plege-token-amount"
	cfgSnapshotFileName     = "snapshot-file"
	cfgSnapshotGenesisSeed  = "seed"
	defaultSnapshotFileName = "./snapshot.bin"
)

// Equally distributed snapshot internal testnet.
var nodesToPledge = map[string]snapshotcreator.Pledge{
	"e3m6WPQXLyuUqEfSHmGVEs6qpyhWNJqtbquX65kFoJQ":  {}, // entrynode
	"EGgbUaAnfXG2mBtGQwSPPVxLa8uC1hnNsxtnLYbHkm8B": {}, // bootstrap_01
	"7PS8tJSjhyFMbUqbVE2pUideT6DQc2ovNv5hBDTkvUtm": {}, // vanilla_01
	"3HqasBLjyqiYWeavLZoi1k1nrMVvGZDGj3EPkKHxzxdZ": {}, // drng_01
	"85LVFFjYZj8JNwmD5BJFux3gVGsw9uT2frFrnQ8gm7dX": {}, // drng_02
	"7Hk4Airu42Gcqm3JZDAL69DSdaksF9qfahppez9LZTJr": {}, // drng_03
	"E3RmVjQHsisxxLY36AuRkV7Uceo1FReYWLMsCTEbDBeC": {}, // drng_04
	"GRbfN6HDzFxWNwN6q4ixmTjDR5oS8XQc5zWbxxFFkBmw": {}, // drng_05
	"12rLUHyF67rzqHgYR6Jxbi3GD5CTU7DaxwDQfmVYcwnV": func() snapshotcreator.Pledge { // faucet_01
		seedBase58 := "D29LzzhHYGPjxtnx3LXFicmLhDVXyhW6379MugJHzSoH" // faucet seed
		seedBytes, err := base58.Decode(seedBase58)
		must(err)
		address := seed.NewSeed(seedBytes).Address(0).Address()
		fmt.Printf("Faucet addr %s", address)
		return snapshotcreator.Pledge{
			Address: address,
		}
	}(),
}

func init() {
	flag.Uint64(cfgGenesisTokenAmount, 800000, "the amount of tokens to add to the genesis output") // we pledge this amount to peer master
	flag.String(cfgSnapshotFileName, defaultSnapshotFileName, "the name of the generated snapshot file")
	// Most recent seed when checking ../integration-tests/assets :
	flag.String(cfgSnapshotGenesisSeed, "7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih", "the genesis seed")
	flag.Uint(cfgPledgeTokenAmount, 1000000000000000, "the amount of tokens to pledge to defined nodes (other than genesis)")
}

func main() {
	flag.Parse()
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}

	snapshotFileName := viper.GetString(cfgSnapshotFileName)
	log.Printf("creating snapshot %s...", snapshotFileName)

	genesisTokenAmount := viper.GetUint64(cfgGenesisTokenAmount)
	pledgeTokenAmount := viper.GetUint64(cfgPledgeTokenAmount)
	seedStr := viper.GetString(cfgSnapshotGenesisSeed)
	seedBytes, err := base58.Decode(seedStr)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to decode base58 seed: %w", err))
	}
	readSnapshot, err2 := snapshotcreator.CreateSnapshot(genesisTokenAmount, seedBytes, pledgeTokenAmount, nodesToPledge, snapshotFileName)
	if err2 != nil {
		log.Fatal("Failed to create snapshot %w", err2)
	}

	printSnapshot(readSnapshot)
}

func printSnapshot(readSnapshot *ledgerstate.Snapshot) {
	fmt.Println("\n================= read Snapshot ===============")
	fmt.Printf("\n================= %d Snapshot Txs ===============\n", len(readSnapshot.Transactions))
	for key, txRecord := range readSnapshot.Transactions {
		fmt.Println("===== key =", key)
		fmt.Println(txRecord)
	}
	fmt.Printf("\n================= %d Snapshot Access Manas ===============\n", len(readSnapshot.AccessManaByNode))
	for key, accessManaNode := range readSnapshot.AccessManaByNode {
		fmt.Println("===== key =", key)
		fmt.Println(accessManaNode)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
