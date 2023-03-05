package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/mr-tron/base58"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock/clockplugin"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

func main() {
	parsedOpts, configSelected, checkValidity := parseFlags()
	opts := BaseOptions
	switch configSelected {
	case "devnet":
		opts = append(opts, Devnet...)
	case "feature":
		opts = append(opts, FeatureNetwork...)
	case "docker":
		opts = append(opts, DockerNetwork...)
	default:
		configSelected = "default"
		opts = BaseOptions
	}
	opts = append(opts, parsedOpts...)
	info := snapshotcreator.NewOptions(opts...)

	log.Printf("creating snapshot with config: %s... %s", configSelected, info.FilePath)
	err := snapshotcreator.CreateSnapshot(opts...)
	if err != nil {
		panic(err)
	}
	if checkValidity {
		diagnosticPrintSnapshotFromFile(info.FilePath, info.SlotTimeProvider)
	}
}

func parseFlags() (opt []options.Option[snapshotcreator.Options], conf string, diagnose bool) {
	filename := flag.String("filename", "", "the name of the generated snapshot file")
	checkValidity := flag.BoolP("diagnose", "d", false, "check the validity of the generated snapshot file")
	config := flag.String("config", "", "use ready config: devnet, feature, docker")
	genesisTokenAmount := flag.Uint64("token-amount", 0, "the amount of tokens to add to the genesis output")
	genesisSeedStr := flag.String("seed", "", "the genesis seed provided in base58 format.")

	flag.Parse()
	opt = []options.Option[snapshotcreator.Options]{}
	if *genesisTokenAmount != 0 {
		opt = append(opt, snapshotcreator.WithGenesisTokenAmount(*genesisTokenAmount))
	}
	if *filename != "" {
		opt = append(opt, snapshotcreator.WithFilePath(*filename))
	}
	if *genesisSeedStr != "" {
		genesisSeed, err := base58.Decode(*genesisSeedStr)
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to decode base58 seed, using the default one"))
		}
		opt = append(opt, snapshotcreator.WithGenesisSeed(genesisSeed))
	}
	return opt, *config, *checkValidity
}

func createTempStorage() (s *storage.Storage) {
	return storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), protocol.DatabaseVersion)
}

func diagnosticPrintSnapshotFromFile(filePath string, provider *slot.TimeProvider) {
	s := createTempStorage()
	defer s.Shutdown()

	e := engine.New(workerpool.NewGroup("Diagnostics"), s, clockplugin.Provide(), dpos.NewProvider(), mana1.NewProvider(), provider)
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
	fmt.Println("SpentOutputs: ")
	if err := e.LedgerState.StateDiffs.StreamSpentOutputs(0, func(owm *ledger.OutputWithMetadata) error {
		fmt.Printf("%d: %+v\n", 0, owm)
		return nil
	}); err != nil {
		panic(err)
	}
	fmt.Println("CreatedOutputs: ")
	if err := e.LedgerState.StateDiffs.StreamCreatedOutputs(0, func(owm *ledger.OutputWithMetadata) error {
		fmt.Printf("%d: %+v\n", 0, owm)
		return nil
	}); err != nil {
		panic(err)
	}
}
