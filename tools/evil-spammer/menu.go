package main

import (
	"fmt"
	"github.com/iotaledger/goshimmer/client/evilwallet"
)

// region Printer /////////////////////////////////////////////////////////////////////////////////////////////////////////

type Printer struct {
	mode *Mode
}

func NewPrinter(mode *Mode) *Printer {
	return &Printer{
		mode: mode,
	}
}

func (p *Printer) Println(s string, indent int) {
	pre := "█"
	for i := 0; i < indent; i++ {
		pre += "▓"
	}
	fmt.Println(pre, s)
}

func (p *Printer) PrintlnPoint(s string, indent int) {
	pre := ""
	for i := 0; i < indent; i++ {
		pre += " "
	}
	fmt.Println(pre, "▀▄", s)
}

func (p *Printer) PrintlnInput(s string) {
	fmt.Println("█▓>>", s)
}
func (p *Printer) PrintThickLine() {
	fmt.Println("\n  ooo▄▄▓░░▀▀▀▀▄▓▓░░▄▄▄▓▓░░▄▒▄▀█▒▓▄▓▓░░▄▄▒▄▄█▒▓▄▄▀▀▄▓▒▄▄█▒▓▓▀▓▓░░░░█▒▄▄█▒▓░▄▄ooo")
	fmt.Println()
}

func (p *Printer) PrintTopLine() {
	fmt.Println("▀▄▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▄▀")
}

func (p *Printer) PrintLine() {
	fmt.Println("▄▀▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▀▄")
}

func (p *Printer) printBanner() {
	fmt.Println("▓█████  ██▒   █▓ ██▓ ██▓                                                   \n▓█   ▀ ▓██░   █▒▓██▒▓██▒                                                   \n▒███    ▓██  █▒░▒██▒▒██░                                                   \n▒▓█  ▄   ▒██ █░░░██░▒██░                                                   \n░▒████▒   ▒▀█░  ░██░░██████▒                                               \n░░ ▒░ ░   ░ ▐░  ░▓  ░ ▒░▓  ░                                               \n ░ ░  ░   ░ ░░   ▒ ░░ ░ ▒  ░                                               \n   ░        ░░   ▒ ░  ░ ░                                                  \n   ░  ░      ░   ░      ░  ░                                               \n            ░                                                              \n           ██████  ██▓███   ▄▄▄       ███▄ ▄███▓ ███▄ ▄███▓▓█████  ██▀███  \n         ▒██    ▒ ▓██░  ██▒▒████▄    ▓██▒▀█▀ ██▒▓██▒▀█▀ ██▒▓█   ▀ ▓██ ▒ ██▒\n         ░ ▓██▄   ▓██░ ██▓▒▒██  ▀█▄  ▓██    ▓██░▓██    ▓██░▒███   ▓██ ░▄█ ▒\n           ▒   ██▒▒██▄█▓▒ ▒░██▄▄▄▄██ ▒██    ▒██ ▒██    ▒██ ▒▓█  ▄ ▒██▀▀█▄  \n         ▒██████▒▒▒██▒ ░  ░ ▓█   ▓██▒▒██▒   ░██▒▒██▒   ░██▒░▒████▒░██▓ ▒██▒\n         ▒ ▒▓▒ ▒ ░▒▓▒░ ░  ░ ▒▒   ▓▒█░░ ▒░   ░  ░░ ▒░   ░  ░░░ ▒░ ░░ ▒▓ ░▒▓░\n         ░ ░▒  ░ ░░▒ ░       ▒   ▒▒ ░░  ░      ░░  ░      ░ ░ ░  ░  ░▒ ░ ▒░\n         ░  ░  ░  ░░         ░   ▒   ░      ░   ░      ░      ░     ░░   ░ \n               ░                 ░  ░       ░          ░      ░  ░   ░     \n                                                                           ")
	p.PrintThickLine()
	p.Println("Interactive mode enabled", 1)
}

func (p *Printer) EvilWalletStatus() {
	p.PrintTopLine()
	p.Println("Evil Wallet status:", 2)
	p.PrintlnPoint(fmt.Sprintf("Available faucet outputs: %d", p.mode.evilWallet.UnspentOutputsLeft(evilwallet.Fresh)), 2)
	p.PrintlnPoint(fmt.Sprintf("Available reuse outputs: %d", p.mode.evilWallet.UnspentOutputsLeft(evilwallet.Reuse)), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed messages: %d", p.mode.msgSent.Load()), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed transactions: %d", p.mode.txSent.Load()), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed conflicts: %d", p.mode.conflictsSent.Load()), 2)

	p.PrintLine()
}

func (p *Printer) SpammerSettings() {
	p.PrintTopLine()
	p.Println("Current settings:", 1)
	p.PrintlnPoint(fmt.Sprintf("Scenario: %s", p.mode.Config.Scenario), 2)
	p.PrintlnPoint(fmt.Sprintf("Deep: %v, Reuse: %v", p.mode.Config.Deep, p.mode.Config.Reuse), 2)
	p.PrintlnPoint(fmt.Sprintf("Rate: %d[mps], Duration: %d[s]", p.mode.Config.Rate, int(p.mode.Config.Duration.Seconds())), 2)
	p.PrintLine()
	p.Println("", 1)
}

func (p *Printer) FarewellMessage() {
	p.PrintTopLine()
	fmt.Println("           GOODBYE... we're forgetting all your private keys ;)")
	p.PrintLine()
}

func (p *Printer) SettingFundsMessage() {
	if p.mode.autoFundsPrepareEnabled {
		p.Println("Auto funds creation enabled", 2)
	} else {
		p.Println("Auto funds creation disabled", 2)
	}
	p.Println("", 1)
}

func (p *Printer) FundsWarning() {
	p.Println("Not enough fresh faucet outputs in the wallet to spam!", 2)
	p.PrintlnPoint("Request more manually with 'Prepare faucet funds' option in main menu.", 2)
	p.PrintlnPoint("You can also enable auto funds requesting in the settings.", 2)
	p.Println("", 1)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
