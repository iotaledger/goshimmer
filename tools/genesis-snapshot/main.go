package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/workerpool"
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
	// "AZKt9NEbNb9TAk5SqVTfj3ANoBzrWLjR5YKxa2BCyi8X", // entrynode
	"BYpRNA5aCuyym8SRFbEATraY4yr9oyuXCsCFVcEM8Fm4", // bootstrap_01
	"5UymiW32h2LM7UqVFf5W1f6iH2DxUqA85RnwP5QgyQYa", // vanilla_01
	"HHPL5wTFjihv7sVHKXYbZkGcDbqq75h1LQntBhKs1saX", // node_01
	"7WAEBePov6Po4kUZFN3h7GNHoddTYTEjhJkmmBPHLW2W", // node_02
	"7xKTSQDtZtiGBAapAh7okHJgnYLq5JJtMUDf2sv1eRrc", // node_03
	"oqSAYKz3v587JG5gRKcnPMnjcG9rVd6jFzJ97pjU5Ms",  // node_04
	"J3Vr2cJ4m85xFGmZa1nda7ZZTWWM9ptYCxrUKXFDAFcc", // node_05
	"3X3ZLueaT6T9mGL8C3YUsDrDqsVYvgbXNsa21jhgdzxi", // faucet_01
}

var initialAttestations = []string{
	"BYpRNA5aCuyym8SRFbEATraY4yr9oyuXCsCFVcEM8Fm4", // bootstrap_01
}

// Devnet
// var nodesToPledge = []string{
// 	//"7Yr1tz7atYcbQUv5njuzoC5MiDsMmr3hqaWtAsgJfxxr", // entrynode
// 	"Gm7W191NDnqyF7KJycZqK7V6ENLwqxTwoKQN4SmpkB24", // bootstrap_01
// 	"9DB3j9cWYSuEEtkvanrzqkzCQMdH1FGv3TawJdVbDxkd", // vanilla_01
// 	"AheLpbhRs1XZsRF8t8VBwuyQh9mqPHXQvthV5rsHytDG", // node_01
// 	"FZ28bSTidszUBn8TTCAT9X1nVMwFNnoYBmZ1xfafez2z", // node_02
// 	"GT3UxryW4rA9RN9ojnMGmZgE2wP7psagQxgVdA4B9L1P", // node_03
// 	"4pB5boPvvk2o5MbMySDhqsmC2CtUdXyotPPEpb7YQPD7", // node_04
// 	"64wCsTZpmKjRVHtBKXiFojw7uw3GszumfvC4kHdWsHga", // node_05
// 	"7DJYaCCnq9bPW2tnwC3BUEDMs6PLDC73NShduZzE4r9k", // faucet_01
// }
//
// var initialAttestations = []string{
//		"Gm7W191NDnqyF7KJycZqK7V6ENLwqxTwoKQN4SmpkB24", // bootstrap_01
// }

// Docker network.
//var nodesToPledge = []string{
//	"EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP", // peer_master
//	"5kjiW423d5EtNh943j8gYahyxPdnv9xge8Kuks5tjoYg", // peer_master2
//	"CHfU1NUf6ZvUKDQHTG2df53GR7CvuMFtyt7YymJ6DwS3", // faucet
//}
//
//var initialAttestations = []string{
//	"EYsaGXnUVA9aTYL9FwYEvoQ8d1HCJveQVL7vogu6pqCP", // peer_master
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

	snapshotcreator.CreateSnapshot(protocol.DatabaseVersion, snapshotFileName, genesisTokenAmount, genesisSeed, manaDistribution, initialAttestationsSlice, new(devnetvm.VM))

	diagnosticPrintSnapshotFromFile(snapshotFileName)
}

func createTempStorage() (s *storage.Storage) {
	return storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), protocol.DatabaseVersion)
}

func createManaDistribution(totalTokensToPledge uint64) (manaDistribution map[ed25519.PublicKey]uint64) {
	manaDistribution = make(map[ed25519.PublicKey]uint64)
	for _, node := range nodesToPledge {
		bytes, err := base58.Decode(node)
		if err != nil {
			panic("failed to decode node public key: " + err.Error())
		}
		nodePublicKey, _, err := ed25519.PublicKeyFromBytes(bytes)
		if err != nil {
			panic("failed to convert bytes to public key: " + err.Error())
		}

		manaDistribution[nodePublicKey] = totalTokensToPledge / uint64(len(nodesToPledge))
	}

	return manaDistribution
}

func createInitialAttestations() (parsed []ed25519.PublicKey) {
	for _, node := range initialAttestations {
		bytes, err := base58.Decode(node)
		if err != nil {
			panic("failed to decode node public key: " + err.Error())
		}
		nodePublicKey, _, err := ed25519.PublicKeyFromBytes(bytes)
		if err != nil {
			panic("failed to convert bytes to public key: " + err.Error())
		}

		parsed = append(parsed, nodePublicKey)
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
	defer s.Shutdown()

	e := engine.New(workerpool.NewGroup("Diagnostics"), s, dpos.NewProvider(), mana1.NewProvider())
	defer e.Shutdown()

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
