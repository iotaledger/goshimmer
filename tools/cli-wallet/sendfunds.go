package main

import (
	"flag"
	"os"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"
	"github.com/mr-tron/base58"
)

func execSendFundsCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	helpPtr := command.Bool("help", false, "show this help screen")
	addressPtr := command.String("dest-addr", "", "destination address for the transfer")
	amountPtr := command.Int64("amount", 0, "the amount of tokens that are supposed to be sent")
	colorPtr := command.String("color", "IOTA", "color of the tokens to transfer (optional)")

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
		printUsage(command, "the amount has to be set and be bigger than 0")
	}
	if *colorPtr == "" {
		printUsage(command, "the color must be set")
	}

	destinationAddress, err := address.FromBase58(*addressPtr)
	if err != nil {
		printUsage(command, err.Error())
	}

	var color balance.Color
	switch *colorPtr {
	case "IOTA":
		color = balance.ColorIOTA
	case "NEW":
		color = balance.ColorNew
	default:
		colorBytes, err := base58.Decode(*colorPtr)
		if err != nil {
			printUsage(command, err.Error())
		}

		color, _, err = balance.ColorFromBytes(colorBytes)
		if err != nil {
			printUsage(command, err.Error())
		}
	}

	_, err = cliWallet.SendFunds(
		wallet.Destination(destinationAddress, uint64(*amountPtr), color),
	)
	if err != nil {
		printUsage(command, err.Error())
	}
}
