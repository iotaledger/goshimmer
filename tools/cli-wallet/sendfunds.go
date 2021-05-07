package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/client/wallet/packages/address"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execSendFundsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	helpPtr := command.Bool("help", false, "show this help screen")
	addressPtr := command.String("dest-addr", "", "destination address for the transfer")
	amountPtr := command.Int64("amount", 0, "the amount of tokens that are supposed to be sent")
	colorPtr := command.String("color", "IOTA", "color of the tokens to transfer (optional)")
	accessManaPledgeIDPtr := command.String("access-mana-id", "", "node ID to pledge access mana to")
	consensusManaPledgeIDPtr := command.String("consensus-mana-id", "", "node ID to pledge consensus mana to")

	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}

	if *helpPtr {
		printUsage(command)
	}

	if *addressPtr == "" {
		printUsage(command, "dest-addr has to be set")
	}
	if *amountPtr <= 0 {
		printUsage(command, "amount has to be set and be bigger than 0")
	}
	if *colorPtr == "" {
		printUsage(command, "color must be set")
	}

	destinationAddress, err := ledgerstate.AddressFromBase58EncodedString(*addressPtr)
	if err != nil {
		printUsage(command, err.Error())
		return
	}

	var color ledgerstate.Color
	switch *colorPtr {
	case "IOTA":
		color = ledgerstate.ColorIOTA
	case "NEW":
		color = ledgerstate.ColorMint
	default:
		colorBytes, parseErr := base58.Decode(*colorPtr)
		if parseErr != nil {
			printUsage(command, parseErr.Error())
		}

		color, _, parseErr = ledgerstate.ColorFromBytes(colorBytes)
		if parseErr != nil {
			printUsage(command, parseErr.Error())
		}
	}

	_, err = cliWallet.SendFunds(
		wallet.Destination(address.Address{
			AddressBytes: destinationAddress.Array(),
		}, uint64(*amountPtr), color),
		wallet.AccessManaPledgeID(*accessManaPledgeIDPtr),
		wallet.ConsensusManaPledgeID(*consensusManaPledgeIDPtr),
	)
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Sending funds ... [DONE]")
}
