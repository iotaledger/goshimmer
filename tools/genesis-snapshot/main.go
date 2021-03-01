package main

import (
	"fmt"
	"log"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/mr-tron/base58"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	cfgGenesisTokenAmount   = "token-amount"
	cfgSnapshotFileName     = "snapshot-file"
	cfgSnapshotGenesisSeed  = "seed"
	defaultSnapshotFileName = "./snapshot.bin"
)

func init() {
	flag.Int(cfgGenesisTokenAmount, 1000000000000000, "the amount of tokens to add to the genesis output")
	flag.String(cfgSnapshotFileName, defaultSnapshotFileName, "the name of the generated snapshot file")
	flag.String(cfgSnapshotGenesisSeed, "", "the genesis seed")
}

func main() {
	flag.Parse()
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}
	genesisTokenAmount := viper.GetInt64(cfgGenesisTokenAmount)
	snapshotFileName := viper.GetString(cfgSnapshotFileName)
	log.Printf("creating snapshot %s...", snapshotFileName)

	seedStr := viper.GetString(cfgSnapshotGenesisSeed)
	if seedStr == "" {
		log.Fatal("Seed is required")
	}
	seedBytes, err := base58.Decode(seedStr)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to decode base58 seed: %w", err))
	}
	genesisSeed := seed.NewSeed(seedBytes)

	mockedConnector := newMockConnector(
		&wallet.Output{
			Address:  genesisSeed.Address(0),
			OutputID: ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0),
			Balances: ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
				ledgerstate.ColorIOTA: 1000000000000000,
			}),
			InclusionState: wallet.InclusionState{
				Liked:     true,
				Confirmed: true,
			},
		},
	)

	genesisWallet := wallet.New(wallet.Import(genesisSeed, 1, []bitmask.BitMask{}, wallet.NewAssetRegistry()), wallet.GenericConnector(mockedConnector))
	genesisAddress := genesisWallet.Seed().Address(0).Address()

	log.Println("genesis:")
	log.Printf("-> seed (base58): %s", genesisWallet.Seed().String())
	log.Printf("-> output address (base58): %s", genesisAddress.Base58())
	log.Printf("-> output id (base58): %s", ledgerstate.NewOutputID(ledgerstate.GenesisTransactionID, 0))
	log.Printf("-> token amount: %d", genesisTokenAmount)

	snapshot := ledgerstate.Snapshot{
		ledgerstate.GenesisTransactionID: {
			genesisAddress: ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: uint64(genesisTokenAmount)}),
		},
	}

	f, err := os.OpenFile(snapshotFileName, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal("unable to create snapshot file", err)
	}
	defer f.Close()

	if _, err = snapshot.WriteTo(f); err != nil {
		log.Fatal("unable to write snapshot content to file", err)
	}

	log.Printf("created %s, bye", snapshotFileName)
}

func (connector *mockConnector) UnspentOutputs(addresses ...address.Address) (outputs map[address.Address]map[ledgerstate.OutputID]*wallet.Output, err error) {
	outputs = make(map[address.Address]map[ledgerstate.OutputID]*wallet.Output)
	for _, addr := range addresses {
		for outputID, output := range connector.outputs[addr] {
			if !output.InclusionState.Spent {
				if _, outputsExist := outputs[addr]; !outputsExist {
					outputs[addr] = make(map[ledgerstate.OutputID]*wallet.Output)
				}

				outputs[addr][outputID] = output
			}
		}
	}

	return
}

type mockConnector struct {
	outputs map[address.Address]map[ledgerstate.OutputID]*wallet.Output
}

func newMockConnector(outputs ...*wallet.Output) (connector *mockConnector) {
	connector = &mockConnector{
		outputs: make(map[address.Address]map[ledgerstate.OutputID]*wallet.Output),
	}

	for _, output := range outputs {
		if _, addressExists := connector.outputs[output.Address]; !addressExists {
			connector.outputs[output.Address] = make(map[ledgerstate.OutputID]*wallet.Output)
		}

		connector.outputs[output.Address][output.OutputID] = output
	}

	return
}

func (connector *mockConnector) RequestFaucetFunds(addr address.Address) (err error) {
	// generate random transaction id

	return
}

func (connector *mockConnector) SendTransaction(tx *ledgerstate.Transaction) (err error) {
	// mark outputs as spent
	return
}
