package main

import (
	"log"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	cfgGenesisTokenAmount   = "token-amount"
	cfgSnapshotFileName     = "snapshot-file"
	defaultSnapshotFileName = "./snapshot.bin"
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

	genesisWallet := wallet.New()
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
