package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/withdrawfundsfromnft_options"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/mr-tron/base58"
)

func execWithdrawFromFTCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	nftIDPtr := command.String("id", "", "unique identifier of the nft to withdraw from")
	addressPtr := command.String("dest-addr", "", "(optional) address to send the withdrew tokens to")
	colorPtr := command.String("color", "IOTA", "color of funds to withdraw")
	amountPtr := command.Int64("amount", 0, "the amount of tokens that are supposed to be withdrew")
	accessManaPledgeIDPtr := command.String("access-mana-id", "", "node ID to pledge access mana to")
	consensusManaPledgeIDPtr := command.String("consensus-mana-id", "", "node ID to pledge consensus mana to")

	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	if *helpPtr {
		printUsage(command)
	}

	if *nftIDPtr == "" {
		printUsage(command, "an nft (alias) ID must be given for withdraw")
	}

	if *amountPtr <= 0 {
		printUsage(command, "amount has to be set and be bigger than 0")
	}
	if *colorPtr == "" {
		printUsage(command, "color must be set")
	}

	aliasID, err := ledgerstate.AliasAddressFromBase58EncodedString(*nftIDPtr)
	if err != nil {
		printUsage(command, err.Error())
	}

	withdrawBalance := map[ledgerstate.Color]uint64{}
	if *amountPtr > 0 {
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
		withdrawBalance[initColor] = uint64(*amountPtr)
	}

	options := []withdrawfundsfromnft_options.WithdrawFundsFromNFTOption{
		withdrawfundsfromnft_options.Alias(aliasID.Base58()),
		withdrawfundsfromnft_options.Amount(withdrawBalance),
		withdrawfundsfromnft_options.AccessManaPledgeID(*accessManaPledgeIDPtr),
		withdrawfundsfromnft_options.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	}

	if *addressPtr != "" {
		address, err := ledgerstate.AddressFromBase58EncodedString(*addressPtr)
		if err != nil {
			printUsage(command, err.Error())
		}
		options = append(options, withdrawfundsfromnft_options.ToAddress(address.Base58()))
	}

	_, err = cliWallet.WithdrawFundsFromNFT(options...)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Withdrawing funds from NFT... [DONE]")
}
