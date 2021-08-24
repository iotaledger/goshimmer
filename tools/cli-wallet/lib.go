package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/capossele/asset-registry/pkg/registryservice"
	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/client/wallet"
	walletseed "github.com/iotaledger/goshimmer/client/wallet/packages/seed"
)

// Exit should be used inside panic intead of os.Exit(). This will allow to call deferred statements.
type Exit struct{ Code int }

func printBanner() {
	fmt.Println("IOTA 2.0 DevNet CLI-Wallet 0.2")
}

func loadWallet() *wallet.Wallet {
	seed, lastAddressIndex, spentAddresses, assetRegistry, err := importWalletStateFile("wallet.dat")
	if err != nil {
		panic(err)
	}

	// configure basic-auth
	options := []client.Option{}
	if config.BasicAuth.IsEnabled() {
		options = append(options, client.WithBasicAuth(config.BasicAuth.Credentials()))
	}

	if assetRegistry != nil {
		// we do have an asset registry parsed
		if config.AssetRegistryNetwork != assetRegistry.Network() && registryservice.Networks[config.AssetRegistryNetwork] {
			assetRegistry = wallet.NewAssetRegistry(config.AssetRegistryNetwork)
		}
	} else if registryservice.Networks[config.AssetRegistryNetwork] {
		// when asset registry is nil, this is the first time that we load the wallet.
		// if config.AssetRegistryNetwork is not valid, we leave assetRegistry as nil, and
		// wallet.New() will initialize it to the default value
		assetRegistry = wallet.NewAssetRegistry(config.AssetRegistryNetwork)
	}

	walletOptions := []wallet.Option{
		wallet.WebAPI(config.WebAPI, options...),
		wallet.Import(seed, lastAddressIndex, spentAddresses, assetRegistry),
	}
	if config.ReuseAddresses {
		walletOptions = append(walletOptions, wallet.ReusableAddress(true))
	}

	walletOptions = append(walletOptions, wallet.FaucetPowDifficulty(config.FaucetPowDifficulty))

	return wallet.New(walletOptions...)
}

func importWalletStateFile(filename string) (seed *walletseed.Seed, lastAddressIndex uint64, spentAddresses []bitmask.BitMask, assetRegistry *wallet.AssetRegistry, err error) {
	walletStateBytes, err := os.ReadFile(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}

		if len(os.Args) < 2 || os.Args[1] != "init" {
			printUsage(nil, "no wallet file (wallet.dat) found: please call \""+filepath.Base(os.Args[0])+" init\"")
		}

		seed = walletseed.NewSeed()
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
	seed = walletseed.NewSeed(seedBytes)
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

	err = os.WriteFile(filename, wallet.ExportState(), 0o644)
	if err != nil {
		panic(err)
	}
}

func printUsage(command *flag.FlagSet, optionalErrorMessage ...string) {
	if len(optionalErrorMessage) >= 1 {
		_, _ = fmt.Fprintf(os.Stderr, "\n")
		_, _ = fmt.Fprintf(os.Stderr, "ERROR:\n  "+optionalErrorMessage[0]+"\n")
	}

	if command == nil {
		fmt.Println()
		fmt.Println("USAGE:")
		fmt.Println("  " + filepath.Base(os.Args[0]) + " [COMMAND]")
		fmt.Println()
		fmt.Println("COMMANDS:")
		fmt.Println("  balance")
		fmt.Println("        show the balances held by this wallet")
		fmt.Println("  send-funds")
		fmt.Println("        initiate a value transfer")
		fmt.Println("  consolidate-funds")
		fmt.Println("        consolidate available funds under one wallet address")
		fmt.Println("  claim-conditional")
		fmt.Println("        claim (move) conditionally owned funds into the wallet")
		fmt.Println("  request-funds")
		fmt.Println("        request funds from the testnet-faucet")
		fmt.Println("  create-asset")
		fmt.Println("        create an asset in the form of colored coins")
		fmt.Println("  asset-info")
		fmt.Println("        returns information about an asset")
		fmt.Println("  delegate-funds")
		fmt.Println("        delegate funds to an address")
		fmt.Println("  reclaim-delegated")
		fmt.Println("        reclaim previously delegated funds")
		fmt.Println("  create-nft")
		fmt.Println("        create an nft as an unforkable alias output")
		fmt.Println("  transfer-nft")
		fmt.Println("        transfer the ownership of an nft")
		fmt.Println("  destroy-nft")
		fmt.Println("        destroy an nft")
		fmt.Println("  deposit-to-nft")
		fmt.Println("        deposit funds into an nft")
		fmt.Println("  withdraw-from-nft")
		fmt.Println("        withdraw funds from an nft")
		fmt.Println("  sweep-nft-owned-funds")
		fmt.Println("        sweep all available funds owned by nft into the wallet")
		fmt.Println("  sweep-nft-owned-nfts")
		fmt.Println("        sweep all available nfts owned by nft into the wallet")
		fmt.Println("  address")
		fmt.Println("        start the address manager of this wallet")
		fmt.Println("  init")
		fmt.Println("        generate a new wallet using a random seed")
		fmt.Println("  server-status")
		fmt.Println("        display the server status")
		fmt.Println("  pledge-id")
		fmt.Println("        query allowed mana pledge nodeIDs")
		fmt.Println("  pending-mana")
		fmt.Println("        display current pending mana of all outputs in the wallet grouped by address")
		fmt.Println("  help")
		fmt.Println("        display this help screen")

		flag.PrintDefaults()

		if len(optionalErrorMessage) >= 1 {
			panic(Exit{1})
		}

		panic(Exit{0})
	}

	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  " + filepath.Base(os.Args[0]) + " " + command.Name() + " [OPTIONS]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	command.PrintDefaults()

	if len(optionalErrorMessage) >= 1 {
		panic(Exit{1})
	}

	panic(Exit{0})
}
