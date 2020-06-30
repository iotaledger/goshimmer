package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			_, _ = fmt.Fprintf(os.Stderr, "\nFATAL ERROR: "+r.(error).Error())
			os.Exit(1)
		}
	}()

	flag.Usage = func() {
		printUsage(nil)
	}

	// load wallet
	wallet := loadWallet()
	defer writeWalletStateFile(wallet, "wallet.dat")

	// check if parameter counts is large enough
	if len(os.Args) < 2 {
		printUsage(nil)
	}

	// define sub commands
	balanceCommand := flag.NewFlagSet("balance", flag.ExitOnError)
	requestFaucetFundsCommand := flag.NewFlagSet("request-funds", flag.ExitOnError)
	addressCommand := flag.NewFlagSet("address", flag.ExitOnError)
	sendFundsCommand := flag.NewFlagSet("send-funds", flag.ExitOnError)
	createAssetCommand := flag.NewFlagSet("create-asset", flag.ExitOnError)

	// switch logic according to provided sub command
	switch os.Args[1] {
	case "balance":
		execBalanceCommand(balanceCommand, wallet)
	case "request-funds":
		execRequestFundsCommand(requestFaucetFundsCommand, wallet)
	case "address":
		execAddressCommand(addressCommand, wallet)
	case "send-funds":
		execSendFundsCommand(sendFundsCommand, wallet)
	case "create-asset":
		execCreateAssetCommand(createAssetCommand, wallet)
	case "init":
		fmt.Println()
		fmt.Println("CREATING WALLET STATE FILE (wallet.dat) ...               [DONE]")
	case "help":
		printUsage(nil)
	default:
		printUsage(nil, "unknown [COMMAND]: "+os.Args[1])
	}
}
