package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/iotaledger/goshimmer/client/wallet"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func execBalanceCommand(command *flag.FlagSet, cliWallet *wallet.Wallet) {
	err := command.Parse(os.Args[2:])
	if err != nil {
		panic(err)
	}
	fmt.Println("Fetching balance...")

	// refresh wallet once
	err = cliWallet.Refresh(true)
	if err != nil {
		printUsage(nil, err.Error())
	}
	// print token balances
	confirmedBalance, pendingBalance, err := cliWallet.AvailableBalance(false)
	if err != nil {
		printUsage(nil, err.Error())
	}

	// initialize tab writer
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	// print header
	fmt.Println()
	fmt.Println("Available Token Balances")
	fmt.Println()
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "STATUS", "BALANCE", "COLOR", "TOKEN NAME")
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "------", "---------------", "--------------------------------------------", "-------------------------")

	// print empty if no balances founds
	if len(confirmedBalance) == 0 && len(pendingBalance) == 0 {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "<EMPTY>", "<EMPTY>", "<EMPTY>", "<EMPTY>")
	} else {
		// print balances
		for color, amount := range confirmedBalance {
			_, _ = fmt.Fprintf(w, "%s\t%d %s\t%s\t%s\n", "[ OK ]", amount, cliWallet.AssetRegistry().Symbol(color), color.String(), cliWallet.AssetRegistry().Name(color))
		}
		for color, amount := range pendingBalance {
			_, _ = fmt.Fprintf(w, "%s\t%d %s\t%s\t%s\n", "[PEND]", amount, cliWallet.AssetRegistry().Symbol(color), color.String(), cliWallet.AssetRegistry().Name(color))
		}
	}
	_ = w.Flush()

	// fetch timelocked balances
	confirmedTimelocked, pendingTimelocked, err := cliWallet.TimelockedBalances(false)
	if err != nil {
		printUsage(nil, err.Error())
	}

	if len(confirmedTimelocked) > 0 || len(pendingTimelocked) > 0 {
		header := "Timelocked Token Balances"
		timeTitle := "LOCKED UNTIL"
		printTimedBalance(header, timeTitle, cliWallet, confirmedTimelocked, pendingTimelocked)
	}

	// fetch conditional balances
	confirmedConditional, pendingConditional, err := cliWallet.ConditionalBalances(false)
	if err != nil {
		printUsage(nil, err.Error())
	}

	if len(confirmedConditional) > 0 || len(pendingConditional) > 0 {
		header := "Conditional Token Balances - execute `claim-conditional` command to sweep these funds into wallet"
		timeTitle := "OWNED UNTIL"
		printTimedBalance(header, timeTitle, cliWallet, confirmedConditional, pendingConditional)
	}

	// fetch balances from wallet
	confirmedGovAliasBalance, confirmedStateAliasBalance, pendingGovAliasBalance, pendingStateAliasBalance, err := cliWallet.AliasBalance(false)
	if err != nil {
		printUsage(nil, err.Error())
	}
	confirmedDel, pendingDel, err := cliWallet.DelegatedAliasBalance(false)
	if err != nil {
		printUsage(nil, err.Error())
	}

	// process returned data, prepare for printing

	// remove alias outputs from confirmedStateAliasBalance that are state & governance controlled
	for aliasID := range confirmedStateAliasBalance {
		if _, has := confirmedGovAliasBalance[aliasID]; has {
			delete(confirmedStateAliasBalance, aliasID)
		}
	}
	// remove alias outputs from pendingStateAliasBalance that are state & governance controlled
	for aliasID := range pendingStateAliasBalance {
		if _, has := pendingGovAliasBalance[aliasID]; has {
			delete(pendingStateAliasBalance, aliasID)
		}
	}

	// remove confirmed delegated alias outputs from governance controlled balance
	for aliasID := range confirmedDel {
		delete(confirmedGovAliasBalance, aliasID)
	}
	// remove pending delegated alias outputs from governance controlled balance
	for aliasID := range pendingDel {
		delete(pendingGovAliasBalance, aliasID)
	}

	if err != nil {
		printUsage(nil, err.Error())
	}

	// first print governed aliases
	if len(confirmedGovAliasBalance) != 0 || len(pendingGovAliasBalance) != 0 {
		// initialize tab writer
		wAlias := new(tabwriter.Writer)
		wAlias.Init(os.Stdout, 0, 8, 2, '\t', 0)

		// print header
		fmt.Println()
		fmt.Println("Owned NFTs (Governance Controlled Aliases)")
		fmt.Println()
		_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%s\t%s\t%s\n", "STATUS", "NFT ID (ALIAS ID)", "BALANCE", "COLOR", "TOKEN NAME")
		_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%s\t%s\t%s\n", "------", "--------------------------------------------", "---------------", "--------------------------------------------", "-------------------------")

		for aliasID, alias := range confirmedGovAliasBalance {
			if _, has := confirmedDel[aliasID]; has {
				continue
			}
			balances := alias.Balances()
			i := 0
			balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				if i == 0 {
					_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%d %s\t%s\t%s\n", "[ OK ]", aliasID.Base58(),
						balance, cliWallet.AssetRegistry().Symbol(color),
						color.String(),
						cliWallet.AssetRegistry().Name(color))
				} else {
					_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%d %s\t%s\t%s\n", "", "", balance,
						cliWallet.AssetRegistry().Symbol(color),
						color.String(),
						cliWallet.AssetRegistry().Name(color))
				}
				i++
				return true
			})
		}
		for aliasID, alias := range pendingGovAliasBalance {
			if _, has := pendingDel[aliasID]; has {
				continue
			}
			balances := alias.Balances()
			i := 0
			balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
				if i == 0 {
					_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%d %s\t%s\t%s\n", "[PEND]", aliasID.Base58(),
						balance, cliWallet.AssetRegistry().Symbol(color),
						color.String(),
						cliWallet.AssetRegistry().Name(color))
				} else {
					_, _ = fmt.Fprintf(wAlias, "%s\t%s\t%d %s\t%s\t%s\n", "", "", balance,
						cliWallet.AssetRegistry().Symbol(color),
						color.String(),
						cliWallet.AssetRegistry().Name(color))
				}
				i++
				return true
			})
		}

		_ = wAlias.Flush()
	}

	// then print only state controlled aliases
	if len(confirmedStateAliasBalance) != 0 || len(pendingStateAliasBalance) != 0 {
		printAliasBalance("Only State Controlled Aliases", "ALIAS ID", cliWallet, confirmedStateAliasBalance, pendingStateAliasBalance)
	}

	// finally print delegated aliases
	if len(confirmedDel) != 0 || len(pendingDel) != 0 {
		printAliasBalance("Delegated Funds", "DELEGATION ID (ALIAS ID)", cliWallet, confirmedDel, pendingDel)
	}
}

func printTimedBalance(header, timeTitle string, cliWallet *wallet.Wallet, confirmed, pending wallet.TimedBalanceSlice) {
	// initialize tab writer
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)
	// defer w.Flush()
	// print header
	fmt.Println()
	fmt.Println(header)
	fmt.Println()
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "STATUS", timeTitle, "BALANCE", "COLOR", "TOKEN NAME")
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "------", "------------------------------", "---------------", "--------------------------------------------", "-------------------------")
	if confirmed != nil {
		confirmed.Sort()
	}
	for _, conditionalBalance := range confirmed {
		// print balances
		for color, amount := range conditionalBalance.Balance {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%d %s\t%s\t%s\n",
				"[ OK ]", conditionalBalance.Time.String(), amount, cliWallet.AssetRegistry().Symbol(color), color.String(),
				cliWallet.AssetRegistry().Name(color))
		}
	}
	if pending != nil {
		pending.Sort()
	}
	for _, conditionalBalance := range pending {
		// print balances
		for color, amount := range conditionalBalance.Balance {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%d %s\t%s\t%s\n",
				"[PEND]", conditionalBalance.Time.String(), amount, cliWallet.AssetRegistry().Symbol(color), color.String(),
				cliWallet.AssetRegistry().Name(color))
		}
	}
	_ = w.Flush()
}

func printAliasBalance(header, idName string, cliWallet *wallet.Wallet, confirmed, pending map[ledgerstate.AliasAddress]*ledgerstate.AliasOutput) {
	// initialize tab writer
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 2, '\t', 0)

	// print header
	fmt.Println()
	fmt.Println(header)
	fmt.Println()
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "STATUS", idName, "BALANCE", "COLOR", "TOKEN NAME")
	_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", "------", "--------------------------------------------", "---------------", "--------------------------------------------", "-------------------------")

	for aliasID, alias := range confirmed {
		balances := alias.Balances()
		i := 0
		balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			if i == 0 {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%d %s\t%s\t%s\n", "[ OK ]", aliasID.Base58(), balance,
					cliWallet.AssetRegistry().Symbol(color),
					color.String(),
					cliWallet.AssetRegistry().Name(color))
			} else {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%d %s\t%s\t%s\n", "", "", balance,
					cliWallet.AssetRegistry().Symbol(color),
					color.String(),
					cliWallet.AssetRegistry().Name(color))
			}
			i++
			return true
		})
	}
	for aliasID, alias := range pending {
		balances := alias.Balances()
		i := 0
		balances.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			if i == 0 {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%d %s\t%s\t%s\n", "[PEND]", aliasID.Base58(), balance,
					cliWallet.AssetRegistry().Symbol(color),
					color.String(),
					cliWallet.AssetRegistry().Name(color))
			} else {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%d %s\t%s\t%s\n", "", "", balance,
					cliWallet.AssetRegistry().Symbol(color),
					color.String(),
					cliWallet.AssetRegistry().Name(color))
			}
			i++
			return true
		})
	}

	_ = w.Flush()
}
