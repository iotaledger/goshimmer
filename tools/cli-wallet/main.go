package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const (
	lockFile = "wallet.LOCK"
)

// entry point for the program
func main() {
	defer func() {
		if r := recover(); r != nil {
			if exit, ok := r.(Exit); ok {
				os.Exit(exit.Code)
			}
			_, _ = fmt.Fprintf(os.Stderr, "\nFATAL ERROR: "+r.(error).Error())
			os.Exit(1)
		}
	}()

	setCWD()

	// Make sure only one instance of the wallet runs
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_RDONLY, 0o600)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err = file.Close(); err != nil {
			panic(err)
		}
		if err = os.Remove(file.Name()); err != nil {
			panic(err)
		}
	}()

	// print banner + initialize framework
	printBanner()
	loadConfig()

	// override Usage to use our custom method
	flag.Usage = func() {
		printUsage(nil)
	}

	// load wallet
	wallet := loadWallet()
	defer writeWalletStateFile(wallet, "wallet.dat")

	// check if parameters potentially include sub commands
	if len(os.Args) < 2 {
		printUsage(nil)
	}

	// define sub commands
	balanceCommand := flag.NewFlagSet("balance", flag.ExitOnError)
	sendFundsCommand := flag.NewFlagSet("send-funds", flag.ExitOnError)
	consolidateFundsCommand := flag.NewFlagSet("consolidate-funds", flag.ExitOnError)
	claimConditionalFundsCommand := flag.NewFlagSet("claim-conditional", flag.ExitOnError)
	createAssetCommand := flag.NewFlagSet("create-asset", flag.ExitOnError)
	assetInfoCommand := flag.NewFlagSet("asset-info", flag.ExitOnError)
	delegateFundsCommand := flag.NewFlagSet("delegate-funds", flag.ExitOnError)
	reclaimDelegatedFundsCommand := flag.NewFlagSet("reclaim-delegated", flag.ExitOnError)
	createNFTCommand := flag.NewFlagSet("create-nft", flag.ExitOnError)
	transferNFTCommand := flag.NewFlagSet("transfer-nft", flag.ExitOnError)
	destroyNFTCommand := flag.NewFlagSet("destroy-nft", flag.ExitOnError)
	depositToNFTCommand := flag.NewFlagSet("deposit-to-nft", flag.ExitOnError)
	withdrawFromNFTCommand := flag.NewFlagSet("withdraw-from-nft", flag.ExitOnError)
	sweepNFTOwnedFundsCommand := flag.NewFlagSet("sweep-nft-owned-funds", flag.ExitOnError)
	sweepNFTOwnedNFTsCommand := flag.NewFlagSet("sweep-nft-owned-nfts", flag.ExitOnError)
	addressCommand := flag.NewFlagSet("address", flag.ExitOnError)
	requestFaucetFundsCommand := flag.NewFlagSet("request-funds", flag.ExitOnError)
	serverStatusCommand := flag.NewFlagSet("server-status", flag.ExitOnError)
	allowedPledgeIDCommand := flag.NewFlagSet("pledge-id", flag.ExitOnError)
	pendingManaCommand := flag.NewFlagSet("pending-mana", flag.ExitOnError)

	// switch logic according to provided sub command
	switch os.Args[1] {
	case "balance":
		execBalanceCommand(balanceCommand, wallet)
	case "address":
		execAddressCommand(addressCommand, wallet)
	case "send-funds":
		execSendFundsCommand(sendFundsCommand, wallet)
	case "consolidate-funds":
		execConsolidateFundsCommand(consolidateFundsCommand, wallet)
	case "claim-conditional":
		execClaimConditionalCommand(claimConditionalFundsCommand, wallet)
	case "create-asset":
		execCreateAssetCommand(createAssetCommand, wallet)
	case "asset-info":
		execAssetInfoCommand(assetInfoCommand, wallet)
	case "delegate-funds":
		execDelegateFundsCommand(delegateFundsCommand, wallet)
	case "reclaim-delegated":
		execReclaimDelegatedFundsCommand(reclaimDelegatedFundsCommand, wallet)
	case "create-nft":
		execCreateNFTCommand(createNFTCommand, wallet)
	case "transfer-nft":
		execTransferNFTCommand(transferNFTCommand, wallet)
	case "destroy-nft":
		execDestroyNFTCommand(destroyNFTCommand, wallet)
	case "deposit-to-nft":
		execDepositToNFTCommand(depositToNFTCommand, wallet)
	case "withdraw-from-nft":
		execWithdrawFromFTCommand(withdrawFromNFTCommand, wallet)
	case "sweep-nft-owned-funds":
		execSweepNFTOwnedFundsCommand(sweepNFTOwnedFundsCommand, wallet)
	case "sweep-nft-owned-nfts":
		execSweepNFTOwnedNFTsCommand(sweepNFTOwnedNFTsCommand, wallet)
	case "request-funds":
		execRequestFundsCommand(requestFaucetFundsCommand, wallet)
	case "pledge-id":
		execAllowedPledgeNodeIDsCommand(allowedPledgeIDCommand, wallet)
	case "pending-mana":
		execPendingMana(pendingManaCommand, wallet)
	case "init":
		fmt.Println()
		fmt.Println("CREATING WALLET STATE FILE (wallet.dat) ...               [DONE]")
	case "server-status":
		execServerStatusCommand(serverStatusCommand, wallet)
	case "help":
		printUsage(nil)
	default:
		printUsage(nil, "unknown [COMMAND]: "+os.Args[1])
	}
}

// ensures the cwd is where the actual go executable
// currently doesn't work well with symlinks (or shortcuts).
func setCWD() {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	dirAbsPath := filepath.Dir(ex)
	if err := os.Chdir(dirAbsPath); err != nil {
		panic(err)
	}
}
