package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// print banner
	fmt.Println("IOTA Pollen CLI-Wallet 0.1")

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
	requestFaucetFundsCommand := flag.NewFlagSet("requestFunds", flag.ExitOnError)
	addressCommand := flag.NewFlagSet("address", flag.ExitOnError)
	sendFundsCommand := flag.NewFlagSet("sendFunds", flag.ExitOnError)

	// switch logic according to provided sub command
	switch os.Args[1] {
	case "balance":
		execBalanceCommand(balanceCommand, wallet)
	case "requestFunds":
		execRequestFundsCommand(requestFaucetFundsCommand, wallet)
	case "address":
		execAddressCommand(addressCommand, wallet)
	case "sendFunds":
		execSendFundsCommand(sendFundsCommand, wallet)
	case "init":
		fmt.Println()
		fmt.Println("CREATING WALLET STATE FILE (wallet.dat) ...               [DONE]")
	case "help":
		printUsage(nil)
	default:
		printUsage(nil, "unknown [COMMAND]: "+os.Args[1])
	}
}
