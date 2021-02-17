package main

import (
	"log"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
	walletaddr "github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	cfgGenesisTokenAmount   = "token-amount"
	cfgSnapshotFileName     = "snapshot-file"
	defaultSnapshotFileName = "./snapshot2.bin"
)

func init() {
	flag.Int(cfgGenesisTokenAmount, 1000000000000000, "the amount of tokens to add to the genesis output")
	flag.String(cfgSnapshotFileName, defaultSnapshotFileName, "the name of the generated snapshot file")
}

func main() {
	flag.Parse()
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}
	genesisTokenAmount := viper.GetInt64(cfgGenesisTokenAmount)
	snapshotFileName := viper.GetString(cfgSnapshotFileName)
	log.Printf("creating snapshot %s...", snapshotFileName)

	genesisSeed := seed.NewSeed()

	mockedConnector := newMockConnector(
		&wallet.Output{
			Address:       genesisSeed.Address(0).Address,
			TransactionID: ledgerstate.GenesisTransactionID,
			Balances: ledgerstate.NewColoredBalances(map[ledgerstate.Color]uint64{
				ledgerstate.ColorIOTA: 1000000000000000,
			}),
			InclusionState: wallet.InclusionState{
				Liked:     true,
				Confirmed: true,
			},
		},
	)

	genesisWallet := wallet.New(wallet.GenericConnector(mockedConnector))
	genesisAddress := genesisWallet.Seed().Address(0).Address

	log.Println("genesis:")
	log.Printf("-> seed (base58): %s", genesisWallet.Seed().String())
	log.Printf("-> output address (base58): %s", genesisAddress.String())
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

func (connector *mockConnector) UnspentOutputs(addresses ...walletaddr.Address) (outputs map[walletaddr.Address]map[ledgerstate.TransactionID]*wallet.Output, err error) {
	outputs = make(map[walletaddr.Address]map[ledgerstate.TransactionID]*wallet.Output)
	for _, addr := range addresses {
		for transactionID, output := range connector.outputs[addr.Address.Base58()] {
			if !output.InclusionState.Spent {
				if _, outputsExist := outputs[addr]; !outputsExist {
					outputs[addr] = make(map[ledgerstate.TransactionID]*wallet.Output)
				}

				outputs[addr][transactionID] = output
			}
		}
	}

	return
}

type mockConnector struct {
	outputs map[string]map[ledgerstate.TransactionID]*wallet.Output
}

func newMockConnector(outputs ...*wallet.Output) (connector *mockConnector) {
	connector = &mockConnector{
		outputs: make(map[string]map[ledgerstate.TransactionID]*wallet.Output),
	}

	for _, output := range outputs {
		if _, addressExists := connector.outputs[output.Address.Base58()]; !addressExists {
			connector.outputs[output.Address.Base58()] = make(map[ledgerstate.TransactionID]*wallet.Output)
		}

		connector.outputs[output.Address.Base58()][output.TransactionID] = output
	}

	return
}

func (connector *mockConnector) RequestFaucetFunds(addr walletaddr.Address) (err error) {
	// generate random transaction id

	return
}

func (connector *mockConnector) SendTransaction(tx *ledgerstate.Transaction) (err error) {
	// mark outputs as spent
	return
}
