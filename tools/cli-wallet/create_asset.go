package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/iotaledger/goshimmer/client/wallet"
)

func execCreateAssetCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	amountPtr := command.Uint64("amount", 0, "the amount of tokens to be created")
	namePtr := command.String("name", "", "the name of the tokens to create")
	symbolPtr := command.String("symbol", "", "the symbol of the tokens to create")

	err := command.Parse(os.Args[2:])
	if err != nil {
		printUsage(command, err.Error())
	}
	if *helpPtr {
		printUsage(command)
	}

	if *amountPtr == 0 {
		printUsage(command)
	}

	if *namePtr == "" {
		printUsage(command, "you need to provide a name for you asset")
	}
	fmt.Println("Creating asset...")
	assetColor, err := cliWallet.CreateAsset(wallet.Asset{
		Name:   *namePtr,
		Symbol: *symbolPtr,
		Supply: *amountPtr,
	})
	if err != nil {
		printUsage(command, err.Error())
	}

	fmt.Println()
	fmt.Println("Creating " + strconv.Itoa(int(*amountPtr)) + " tokens with the color '" + assetColor.String() + "' ...   [DONE]")
}
