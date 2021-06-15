package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/createnftoptions"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execCreateNFTCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	initialAmountPtr := command.Int64("initial-amount", 0, "the amount of tokens that should be deposited into the nft upon creation (on top of the minimum required)")
	colorPtr := command.String("color", "IOTA", "color of the tokens that should be deposited into the nft upon creation (on top of the minimum required)")
	immutableDataFile := command.String("immutable-data", "", "path to the file containing the immutable data that shall be attached to the nft")
	accessManaPledgeIDPtr := command.String("access-mana-id", "", "node ID to pledge access mana to")
	consensusManaPledgeIDPtr := command.String("consensus-mana-id", "", "node ID to pledge consensus mana to")

	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	if *helpPtr {
		printUsage(command)
	}

	if *initialAmountPtr < 0 {
		printUsage(command, "initial funding amount cannot be less than 0")
	}
	if *colorPtr == "" {
		printUsage(command, "color must be set")
	}
	var data []byte
	if *immutableDataFile != "" {
		file, fErr := os.Open(*immutableDataFile)
		if fErr != nil {
			printUsage(command, fErr.Error())
		}
		defer file.Close()

		data, err = ioutil.ReadAll(file)
		if err != nil {
			printUsage(command, err.Error())
		}
	}

	initialBalance := map[ledgerstate.Color]uint64{ledgerstate.ColorIOTA: ledgerstate.DustThresholdAliasOutputIOTA}
	if *initialAmountPtr > 0 {
		var initColor ledgerstate.Color
		// get color
		switch *colorPtr {
		case "IOTA":
			initColor = ledgerstate.ColorIOTA
		case "NEW":
			initColor = ledgerstate.ColorMint
		default:
			colorBytes, parseErr := base58.Decode(*colorPtr)
			if parseErr != nil {
				printUsage(command, parseErr.Error())
			}

			initColor, _, parseErr = ledgerstate.ColorFromBytes(colorBytes)
			if parseErr != nil {
				printUsage(command, parseErr.Error())
			}
		}
		initialBalance[initColor] = uint64(*initialAmountPtr)
	}

	options := []createnftoptions.CreateNFTOption{
		createnftoptions.InitialBalance(initialBalance),
		createnftoptions.AccessManaPledgeID(*accessManaPledgeIDPtr),
		createnftoptions.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	}
	if data != nil {
		options = append(options, createnftoptions.ImmutableData(data))
	}
	fmt.Println("Creating NFT...")
	_, nftID, err := cliWallet.CreateNFT(options...)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Created NFT with ID: ", nftID.Base58())
	fmt.Println("Creating NFT... [DONE]")
}
