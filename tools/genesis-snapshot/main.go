package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
)

const (
	cfgGenesisTokenAmount     = "token-amount"
	cfgPledgeTokenAmount      = "plege-token-amount"
	cfgSnapshotFileName       = "snapshot-file"
	cfgSnapshotGenesisSeed    = "seed"
	defaultSnapshotFileName   = "./snapshot.bin"
	defaultGenesisTokenAmount = 1000000000000000
)

// Feature network.
var nodesToPledge = []string{
	"Xv5Kmv9uZfNME4KD2zBoHZ3kVqovJN59ec62rH3AeLA",  // entrynode
	"EUq4re4sZBMbmzdKo8LJF8uVQhbS24ZNeLRf7AntGH7b", // bootstrap_01
	"6PqeR7gpR9KtVt7ZgxrEnTj76TS7S439R1gmmzLLrBcU", // vanilla_01
	"DmKUMcbs6go8sMhJLfZxL8NKXHtYdxQMwVjyacsw4c6C", // node_01
	"GCvqziTVeHHvM4SvSeLBobY2KYTNiyB1miS9giVDgJbk", // node_02
	"64RCLnQC7ECpHGpq7dWp3Xtpc79uVi61XYBv5fgXsD9h", // node_03
	"Amkmn4nt8qwboUGPmFhCoM9ogeCbvS3eBSTjuoE3a5ci", // node_04
	"4FJbEsv448BoXeRo1a5Cq9xizkP2AkRBcx9W4PwDt2GL", // node_05
	"GbkZ3CoiTuUPUAYjgZLM8Y1VgvUbPujHVxmYmPVY2GDC", // faucet_01
}

var initialAttestations = []string{
	"EUq4re4sZBMbmzdKo8LJF8uVQhbS24ZNeLRf7AntGH7b", // bootstrap_01
}

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
//
//var initialAttestations = []string{
//		"AuQXPFmRu9nKNtUq3g1RLqVgSmxNrYeogt6uRwqYLGvK", // bootstrap_01
//}

// Docker network.
//var nodesToPledge = []string{
//	"2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5", // peer_master
//	"AXSoTPcN6SNwH64tywpz4k2XfAc24NR7ckKX8wPjeUZD", // peer_master2
//	"FZ6xmPZXRs2M8z9m9ETTQok4PCga4X8FRHwQE6uYm4rV", // faucet
//}
//
//var initialAttestations = []string{
//	"2GtxMQD94KvDH1SJPJV7icxofkyV1njuUZKtsqKmtux5", // peer_master
//}

func main() {
	snapshotFileName := viper.GetString(cfgSnapshotFileName)
	log.Printf("creating createdSnapshot %s...", snapshotFileName)

	genesisTokenAmount := viper.GetUint64(cfgGenesisTokenAmount)
	totalTokensToPledge := viper.GetUint64(cfgPledgeTokenAmount)
	genesisSeed, err := base58.Decode(viper.GetString(cfgSnapshotGenesisSeed))
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to decode base58 seed"))
	}

	manaDistribution := createManaDistribution(totalTokensToPledge)
	initialAttestationsSlice := createInitialAttestations()

	snapshotcreator.CreateSnapshot(protocol.DatabaseVersion, snapshotFileName, genesisTokenAmount, genesisSeed, manaDistribution, initialAttestationsSlice)

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

func createInitialAttestations() (parsed []identity.ID) {
	for _, node := range initialAttestations {
		nodeID, err := identity.DecodeIDBase58(node)
		if err != nil {
			panic("failed to decode node id: " + err.Error())
		}

		parsed = append(parsed, nodeID)
	}

	return parsed
}

func init() {
	flag.Uint64(cfgGenesisTokenAmount, defaultGenesisTokenAmount, "the amount of tokens to add to the genesis output") // this amount is pledged to the empty nodeID.
	flag.String(cfgSnapshotFileName, defaultSnapshotFileName, "the name of the generated snapshot file")
	flag.String(cfgSnapshotGenesisSeed, "7R1itJx5hVuo9w9hjg5cwKFmek4HMSoBDgJZN8hKGxih", "the genesis seed")
	flag.Uint(cfgPledgeTokenAmount, defaultGenesisTokenAmount, "the amount of tokens to pledge to defined nodes (other than genesis)")

	flag.Parse()
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}
}

func diagnosticPrintSnapshotFromFile(filePath string) {
	s := createTempStorage()
	e := engine.New(s, dpos.NewProvider(), mana1.NewProvider())
	if err := e.Initialize(filePath); err != nil {
		panic(err)
	}

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
	if err := e.Storage.RootBlocks.Stream(0, func(blockID models.BlockID) (err error) {
		fmt.Printf("%+v\n", blockID)

		return
	}); err != nil {
		panic(err)
	}

	fmt.Println("--- ActivityLog ---")
	if err := lo.PanicOnErr(e.NotarizationManager.Attestations.Get(0)).Stream(func(id identity.ID, attestation *notarization.Attestation) bool {
		fmt.Printf("%d: %+v\n", 0, id)
		fmt.Printf("Attestation: %+v\n", attestation)
		return true
	}); err != nil {
		panic(err)
	}

	fmt.Println("--- Diffs ---")
	if err := e.LedgerState.StateDiffs.StreamSpentOutputs(0, func(owm *ledger.OutputWithMetadata) error {
		fmt.Printf("%d: %+v\n", 0, owm)
		return nil
	}); err != nil {
		panic(err)
	}

	if err := e.LedgerState.StateDiffs.StreamCreatedOutputs(0, func(owm *ledger.OutputWithMetadata) error {
		fmt.Printf("%d: %+v\n", 0, owm)
		return nil
	}); err != nil {
		panic(err)
	}
}
