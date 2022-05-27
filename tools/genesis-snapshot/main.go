package main

import (
	"fmt"
	"log"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/snapshot"
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

// // consensus integration test snapshot, use with cfgGenesisTokenAmount=800000 for mana distribution: 50%, 25%, 25%
// var nodesToPledge = map[string]Pledge{
//	// peer master
//	"EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP": {Genesis: true},
//	// "CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3": {}, // faucet
//	// base58:Bk69VaYsRuiAaKn8hK6KxUj45X5dED3ueRtxfYnsh4Q8
//	"3kwsHfLDb7ifuxLbyMZneXq3s5heRWnXKKGPAARJDaUE": func() Pledge {
//		seedBase58 := "CFE7T9hjePFwg2P3Mqf5ELH3syFMReVbWai6qc4rJsff"
//		seedBytes, err := base58.Decode(seedBase58)
//		must(err)
//		return Pledge{
//			Address: seed.NewSeed(seedBytes).Address(0).Address(),
//			Amount:  1600000,
//		}
//	}(),
//	// base58:HUH4rmxUxMZBBtHJ4QM5Ts6s8DP3HnFpChejntnCxto2
//	"9fC9crffh3xYuw3M114ZtxRFxxCFceG8vdq2RAjDVQCK": func() Pledge {
//		seedBase58 := "5qm7UPdKKv3GqHyUQgHX3eS1VwdNsWEr2JWqe2GjDZx3"
//		seedBytes, err := base58.Decode(seedBase58)
//		must(err)
//		return Pledge{
//			Address: seed.NewSeed(seedBytes).Address(0).Address(),
//			Amount:  800000,
//		}
//	}(),
// }

// Equally distributed snapshot internal testnet.
// var nodesToPledge = []string{
// 	"e3m6WPQXLyuUqEfSHmGVEs6qpyhWNJqtbquX65kFoJQ",  // entrynode
// 	"EGgbUaAnfXG2mBtGQwSPPVxLa8uC1hnNsxtnLYbHkm8B", // bootstrap_01
// 	"7PS8tJSjhyFMbUqbVE2pUideT6DQc2ovNv5hBDTkvUtm", // vanilla_01
// 	"3HqasBLjyqiYWeavLZoi1k1nrMVvGZDGj3EPkKHxzxdZ", // drng_01
// 	"85LVFFjYZj8JNwmD5BJFux3gVGsw9uT2frFrnQ8gm7dX", // drng_02
// 	"7Hk4Airu42Gcqm3JZDAL69DSdaksF9qfahppez9LZTJr", // drng_03
// 	"E3RmVjQHsisxxLY36AuRkV7Uceo1FReYWLMsCTEbDBeC", // drng_04
// 	"GRbfN6HDzFxWNwN6q4ixmTjDR5oS8XQc5zWbxxFFkBmw", // drng_05
// 	"12rLUHyF67rzqHgYR6Jxbi3GD5CTU7DaxwDQfmVYcwnV", // faucet_01
// }

// Feature network.
// var nodesToPledge = []string{
// 	"Xv5Kmv9uZfNME4KD2zBoHZ3kVqovJN59ec62rH3AeLA",  // entrynode
// 	"EUq4re4sZBMbmzdKo8LJF8uVQhbS24ZNeLRf7AntGH7b", // bootstrap_01
// 	"6PqeR7gpR9KtVt7ZgxrEnTj76TS7S439R1gmmzLLrBcU", // vanilla_01
// 	"DmKUMcbs6go8sMhJLfZxL8NKXHtYdxQMwVjyacsw4c6C", // drng_01
// 	"GCvqziTVeHHvM4SvSeLBobY2KYTNiyB1miS9giVDgJbk", // drng_02
// 	"64RCLnQC7ECpHGpq7dWp3Xtpc79uVi61XYBv5fgXsD9h", // drng_03
// 	"Amkmn4nt8qwboUGPmFhCoM9ogeCbvS3eBSTjuoE3a5ci", // drng_04
// 	"4FJbEsv448BoXeRo1a5Cq9xizkP2AkRBcx9W4PwDt2GL", // drng_05
// 	"GbkZ3CoiTuUPUAYjgZLM8Y1VgvUbPujHVxmYmPVY2GDC", // faucet_01
// }

// Docker network.
var nodesToPledge = []string{
	"2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5", // peer_master
	"AXSoTPcN6SNwH64tywpz4k2XfAc24NR7ckKX8wPjeUZD", // peer_master2
	"FZ6xmPZXRs2M8z9m9ETTQok4PCga4X8FRHwQE6uYm4rV", // faucet
}

func main() {
	snapshotFileName := viper.GetString(cfgSnapshotFileName)
	log.Printf("creating createdSnapshot %s...", snapshotFileName)

	genesisTokenAmount := viper.GetUint64(cfgGenesisTokenAmount)
	totalTokensToPledge := viper.GetUint64(cfgPledgeTokenAmount)
	genesisSeed, err := base58.Decode(viper.GetString(cfgSnapshotGenesisSeed))
	if err != nil {
		log.Fatal(fmt.Errorf("failed to decode base58 seed: %w", err))
	}

	manaDistribution := createManaDistribution(totalTokensToPledge)

	createdSnapshot, err := snapshotcreator.CreateSnapshot(genesisTokenAmount, genesisSeed, manaDistribution)
	if err != nil {
		log.Fatal("Failed to create createdSnapshot %w", err)
	}

	if err = createdSnapshot.WriteFile(snapshotFileName); err != nil {
		panic(err)
	}

	fmt.Println("created", createdSnapshot)

	u := new(snapshot.Snapshot)
	err = u.FromBytes(createdSnapshot.Bytes())
	if err != nil {
		panic(err)
	}

	fmt.Println("unmarshalled", u.LedgerSnapshot)
}

func createManaDistribution(totalTokensToPledge uint64) (manaDistribution map[identity.ID]uint64) {
	manaDistribution = make(map[identity.ID]uint64)
	for _, node := range nodesToPledge {
		nodeID, err := identity.DecodeIDBase58(node)
		if err != nil {
			panic("failed to decode node id: " + err.Error())
		}

		manaDistribution[nodeID] = totalTokensToPledge / uint64(len(nodesToPledge))
	}

	return manaDistribution
}

func init() {
	flag.Uint64(cfgGenesisTokenAmount, 1000000000000000, "the amount of tokens to add to the genesis output") // this amount is pledged to the empty nodeID.
	flag.String(cfgSnapshotFileName, defaultSnapshotFileName, "the name of the generated snapshot file")
	flag.String(cfgSnapshotGenesisSeed, "7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih", "the genesis seed")
	flag.Uint(cfgPledgeTokenAmount, 1000000000000000, "the amount of tokens to pledge to defined nodes (other than genesis)")

	flag.Parse()
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}
}
