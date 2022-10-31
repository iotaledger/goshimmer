package main

import (
	"fmt"
	"log"
	"os"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/mr-tron/base58"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/core/snapshot/creator"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	ledgerModels "github.com/iotaledger/goshimmer/packages/storage/models"
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

// Feature network.
// var nodesToPledge = []string{
// 	"Xv5Kmv9uZfNME4KD2zBoHZ3kVqovJN59ec62rH3AeLA",  // entrynode
// 	"EUq4re4sZBMbmzdKo8LJF8uVQhbS24ZNeLRf7AntGH7b", // bootstrap_01
// 	"6PqeR7gpR9KtVt7ZgxrEnTj76TS7S439R1gmmzLLrBcU", // vanilla_01
// 	"DmKUMcbs6go8sMhJLfZxL8NKXHtYdxQMwVjyacsw4c6C", // node_01
// 	"GCvqziTVeHHvM4SvSeLBobY2KYTNiyB1miS9giVDgJbk", // node_02
// 	"64RCLnQC7ECpHGpq7dWp3Xtpc79uVi61XYBv5fgXsD9h", // node_03
// 	"Amkmn4nt8qwboUGPmFhCoM9ogeCbvS3eBSTjuoE3a5ci", // node_04
// 	"4FJbEsv448BoXeRo1a5Cq9xizkP2AkRBcx9W4PwDt2GL", // node_05
// 	"GbkZ3CoiTuUPUAYjgZLM8Y1VgvUbPujHVxmYmPVY2GDC", // faucet_01
// }

// Devnet
// var nodesToPledge = []string{
// 	"7Yr1tz7atYcbQUv5njuzoC5MiDsMmr3hqaWtAsgJfxxr", // entrynode
// 	"AuQXPFmRu9nKNtUq3g1RLqVgSmxNrYeogt6uRwqYLGvK", // bootstrap_01
// 	"D9SPFofAGhA5V9QRDngc1E8qG9bTrnATmpZMdoyRiBoW", // vanilla_01
// 	"CfkVFzXRjJdshjgPpQAZ4fccZs2SyVPGkTc8LmtnbsT",  // node_01
// 	"AQfLfcKpvt1nWn916ZGSBy7bRPkjEv5sN7fSZ2rFKoPh", // node_02
// 	"9GLqh2VaDYUiKGn7kwV2EXsnU6Eiv7AEW73bXhfnX6FD", // node_03
// 	"C3VeWTBAi12JHXKWTvYCxBRyVya6UpbdziiGZNqgh1sB", // node_04
// 	"HGiFs4jR74yxDMCN8K1Z16QPdwchXokXzYhBLqKW2ssW", // node_05
// 	"5heLsHxMRdTewXooaaDFGpAoj5c41ah5wTmpMukjdvi7", // faucet_01
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

	creator.CreateSnapshot(createTempStorage(), snapshotFileName, genesisTokenAmount, genesisSeed, manaDistribution)

	diagnosticPrintSnapshotFromFile(snapshotFileName)
}

func createTempStorage() (s *storage.Storage) {
	return storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), protocol.DatabaseVersion)
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

func diagnosticPrintSnapshotFromFile(filePath string) {
	s := createTempStorage()
	e := engine.New(s)
	fileHandle := lo.PanicOnErr(os.Open(filePath))

	snapshot.ReadSnapshot(fileHandle, e)

	fmt.Println("--- Settings ---")
	fmt.Printf("%+v\n", s.Settings)

	fmt.Println("--- Commitments ---")
	fmt.Printf("%+v\n", lo.PanicOnErr(s.Commitments.Load(0)))

	fmt.Println("--- Ledgerstate ---")
	e.Ledger.Storage.ForEachOutputID(func(outputID utxo.OutputID) bool {
		e.Ledger.Storage.CachedOutput(outputID).Consume(func(o utxo.Output) {
			e.Ledger.Storage.CachedOutputMetadata(outputID).Consume(func(m *ledger.OutputMetadata) {
				fmt.Printf("%+v\n%#v\n", o, m)
			})
		})
		return true
	})

	fmt.Println("--- SEPs ---")
	e.Storage.SolidEntryPoints.Stream(0, func(b *models.Block) {
		fmt.Printf("%+v\n", b)
	})

	fmt.Println("--- ActivityLog ---")
	e.Storage.ActiveNodes.Stream(0, func(id identity.ID) {
		fmt.Printf("%d: %+v\n", 0, id)
	})

	fmt.Println("--- Diffs ---")
	e.Storage.LedgerStateDiffs.StreamSpentOutputs(0, func(owm *ledgerModels.OutputWithMetadata) {
		fmt.Printf("%d: %+v\n", 0, owm)
	})
	e.Storage.LedgerStateDiffs.StreamCreatedOutputs(0, func(owm *ledgerModels.OutputWithMetadata) {
		fmt.Printf("%d: %+v\n", 0, owm)
	})
}
