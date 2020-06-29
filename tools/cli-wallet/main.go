package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// print banner
	fmt.Println("IOTA Pollen CLI-Wallet 0.1")
	fmt.Println()

	// load wallet
	wallet := loadWallet()
	defer writeWalletStateFile(wallet, "wallet.dat")

	// check if parameter counts is large enough
	if len(os.Args) < 2 {
		printUsage()
	}

	// define sub commands
	balanceCommand := flag.NewFlagSet("balance", flag.ExitOnError)
	requestFaucetFundsCommand := flag.NewFlagSet("list", flag.ExitOnError)

	// switch logic according to provided sub command
	switch os.Args[1] {
	case "balance":
		execBalanceCommand(balanceCommand, wallet)
	case "requestFunds":
		execRequestFundsCommand(requestFaucetFundsCommand, wallet)
	case "init":
		fmt.Println()
		fmt.Println("GENERATING NEW WALLET (wallet.dat) ...                    [DONE]")
	case "help":
		printUsage()
	default:
		printUsage("unknown [COMMAND]: " + os.Args[1])
	}
}
