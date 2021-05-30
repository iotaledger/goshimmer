package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execAssetInfoCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	command.Usage = func() {
		printUsage(command)
	}

	helpPtr := command.Bool("help", false, "show this help screen")
	assetID := command.String("id", "", "the assetID (color) of the asset to be checked")

	err := command.Parse(os.Args[2:])
	if err != nil {
		printUsage(command, err.Error())
	}
	if *helpPtr {
		printUsage(command)
	}

	if *assetID == "" {
		printUsage(command, "you need to provide an assetID (color)")
	}

	color, err := ledgerstate.ColorFromBase58EncodedString(*assetID)
	if err != nil {
		printUsage(command, fmt.Sprintf("wrong assetID (color) provided: %s", err.Error()))
	}

	asset, err := cliWallet.AssetRegistry().LoadAsset(color)
	if err != nil {
		printUsage(command, fmt.Sprintf("failed to fetch asset info: %s", err.Error()))
	}

	fmt.Println()
	fmt.Println("Asset Info")
	fmt.Println()
	// initialize tab writer
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	_, _ = fmt.Fprintf(w, "%s\t%s\n", "PROPERTY", "VALUE")
	_, _ = fmt.Fprintf(w, "%s\t%s\n", "-----------------------", "--------------------------------------------")
	_, _ = fmt.Fprintf(w, "%s\t%s\n", "Name", asset.Name)
	_, _ = fmt.Fprintf(w, "%s\t%s\n", "Symbol", asset.Symbol)
	_, _ = fmt.Fprintf(w, "%s\t%s\n", "AssetID(color)", asset.Color.Base58())
	_, _ = fmt.Fprintf(w, "%s\t%d\n", "Initial Supply", asset.Supply)
	_, _ = fmt.Fprintf(w, "%s\t%s\n", "Creating Transaction", asset.TransactionID.Base58())
	_, _ = fmt.Fprintf(w, "%s\t%s\n", "Network", cliWallet.AssetRegistry().Network())

	_ = w.Flush()
}
