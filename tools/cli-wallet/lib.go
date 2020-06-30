package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

func printBanner() {
	fmt.Println("IOTA Pollen CLI-Wallet 0.1")
}

func loadWallet() *wallet.Wallet {
	seed, lastAddressIndex, spentAddresses, assetRegistry, err := importWalletStateFile("wallet.dat")
	if err != nil {
		panic(err)
	}

	return wallet.New(
		wallet.WebAPI(config.WebAPI),
		wallet.Import(seed, lastAddressIndex, spentAddresses, assetRegistry),
	)
}

func importWalletStateFile(filename string) (seed *wallet.Seed, lastAddressIndex uint64, spentAddresses []bitmask.BitMask, assetRegistry *wallet.AssetRegistry, err error) {
	walletStateBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}

		if len(os.Args) < 2 || os.Args[1] != "init" {
			printUsage(nil, "no wallet file (wallet.dat) found: please call "+filepath.Base(os.Args[0])+" init")
		}

		seed = wallet.NewSeed()
		lastAddressIndex = 0
		spentAddresses = []bitmask.BitMask{}
		err = nil

		fmt.Println("GENERATING NEW WALLET ...                                 [DONE]")
		fmt.Println()
		fmt.Println("================================================================")
		fmt.Println("!!!            PLEASE CREATE A BACKUP OF YOUR SEED           !!!")
		fmt.Println("!!!                                                          !!!")
		fmt.Println("!!!       " + base58.Encode(seed.Bytes()) + "       !!!")
		fmt.Println("!!!                                                          !!!")
		fmt.Println("!!!            PLEASE CREATE A BACKUP OF YOUR SEED           !!!")
		fmt.Println("================================================================")

		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "init" {
		printUsage(nil, "please remove the wallet.dat before trying to create a new wallet")
	}

	marshalUtil := marshalutil.New(walletStateBytes)

	seedBytes, err := marshalUtil.ReadBytes(ed25519.SeedSize)
	seed = wallet.NewSeed(seedBytes)
	if err != nil {
		return
	}

	lastAddressIndex, err = marshalUtil.ReadUint64()
	if err != nil {
		return
	}

	assetRegistry, _, err = wallet.ParseAssetRegistry(marshalUtil)

	spentAddressesBytes := marshalUtil.ReadRemainingBytes()
	spentAddresses = *(*[]bitmask.BitMask)(unsafe.Pointer(&spentAddressesBytes))

	return
}

func writeWalletStateFile(wallet *wallet.Wallet, filename string) {
	var skipRename bool
	info, err := os.Stat(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}

		skipRename = true
	}
	if err == nil && info.IsDir() {
		panic("found directory instead of file at " + filename)
	}

	if !skipRename {
		err = os.Rename(filename, filename+".bkp")
		if err != nil && os.IsNotExist(err) {
			panic(err)
		}
	}

	err = ioutil.WriteFile(filename, wallet.ExportState(), 0644)
	if err != nil {
		panic(err)
	}
}

func printUsage(command *flag.FlagSet, optionalErrorMessage ...string) {
	if len(optionalErrorMessage) >= 1 {
		_, _ = fmt.Fprintf(os.Stderr, "\n")
		_, _ = fmt.Fprintf(os.Stderr, optionalErrorMessage[0]+"\n")
	}

	if command == nil {
		fmt.Println()
		fmt.Println("USAGE:")
		fmt.Println("  " + filepath.Base(os.Args[0]) + " [COMMAND]")
		fmt.Println()
		fmt.Println("COMMANDS:")
		fmt.Println("  init")
		fmt.Println("  balance")
		fmt.Println("  address")
		fmt.Println("  send-funds")
		fmt.Println("  create-asset")
		fmt.Println("  request-funds")
		fmt.Println("  help")

		flag.PrintDefaults()

		if len(optionalErrorMessage) >= 1 {
			os.Exit(1)
		}

		os.Exit(0)
	}

	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  " + filepath.Base(os.Args[0]) + " " + command.Name() + " [OPTIONS]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	command.PrintDefaults()

	if len(optionalErrorMessage) >= 1 {
		os.Exit(1)
	}

	os.Exit(0)
}
