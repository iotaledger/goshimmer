package main

import (
	"fmt"
	"os"
	"time"

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
	fmt.Println("▀▄---------------------------------------------------------------------------▄▀")
}

func (p *Printer) PrintLine() {
	fmt.Println("▄▀___________________________________________________________________________▀▄")
}

func (p *Printer) printBanner() {
	fmt.Println("▓█████  ██▒   █▓ ██▓ ██▓                                                   \n▓█   ▀ ▓██░   █▒▓██▒▓██▒                                                   \n▒███    ▓██  █▒░▒██▒▒██░                                                   \n▒▓█  ▄   ▒██ █░░░██░▒██░                                                   \n░▒████▒   ▒▀█░  ░██░░██████▒                                               \n░░ ▒░ ░   ░ ▐░  ░▓  ░ ▒░▓  ░                                               \n ░ ░  ░   ░ ░░   ▒ ░░ ░ ▒  ░                                               \n   ░        ░░   ▒ ░  ░ ░                                                  \n   ░  ░      ░   ░      ░  ░                                               \n            ░                                                              \n           ██████  ██▓███   ▄▄▄       ███▄ ▄███▓ ███▄ ▄███▓▓█████  ██▀███  \n         ▒██    ▒ ▓██░  ██▒▒████▄    ▓██▒▀█▀ ██▒▓██▒▀█▀ ██▒▓█   ▀ ▓██ ▒ ██▒\n         ░ ▓██▄   ▓██░ ██▓▒▒██  ▀█▄  ▓██    ▓██░▓██    ▓██░▒███   ▓██ ░▄█ ▒\n           ▒   ██▒▒██▄█▓▒ ▒░██▄▄▄▄██ ▒██    ▒██ ▒██    ▒██ ▒▓█  ▄ ▒██▀▀█▄  \n         ▒██████▒▒▒██▒ ░  ░ ▓█   ▓██▒▒██▒   ░██▒▒██▒   ░██▒░▒████▒░██▓ ▒██▒\n         ▒ ▒▓▒ ▒ ░▒▓▒░ ░  ░ ▒▒   ▓▒█░░ ▒░   ░  ░░ ▒░   ░  ░░░ ▒░ ░░ ▒▓ ░▒▓░\n         ░ ░▒  ░ ░░▒ ░       ▒   ▒▒ ░░  ░      ░░  ░      ░ ░ ░  ░  ░▒ ░ ▒░\n         ░  ░  ░  ░░         ░   ▒   ░      ░   ░      ░      ░     ░░   ░ \n               ░                 ░  ░       ░          ░      ░  ░   ░     \n                                                                           ")
	p.PrintThickLine()
	p.Println("Interactive mode enabled", 1)
	fmt.Println()
}

func (p *Printer) EvilWalletStatus() {
	p.PrintTopLine()
	p.Println(p.colorString("Evil Wallet status:", "cyan"), 2)
	p.PrintlnPoint(fmt.Sprintf("Available faucet outputs: %d", p.mode.evilWallet.UnspentOutputsLeft(evilwallet.Fresh)), 2)
	p.PrintlnPoint(fmt.Sprintf("Available reuse outputs: %d", p.mode.evilWallet.UnspentOutputsLeft(evilwallet.Reuse)), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed messages: %d", p.mode.msgSent.Load()), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed transactions: %d", p.mode.txSent.Load()), 2)
	p.PrintlnPoint(fmt.Sprintf("Spammed scenario batches: %d", p.mode.scenariosSent.Load()), 2)

	p.PrintLine()
	fmt.Println()
}

func (p *Printer) SpammerSettings() {
	p.PrintTopLine()
	p.Println(p.colorString("Current settings:", "cyan"), 1)
	p.PrintlnPoint(fmt.Sprintf("Scenario: %s", p.mode.Config.Scenario), 2)
	p.PrintlnPoint(fmt.Sprintf("Deep: %v, Reuse: %v", p.mode.Config.Deep, p.mode.Config.Reuse), 2)
	p.PrintlnPoint(fmt.Sprintf("Rate: %d[mps], Duration: %d[s]", p.mode.Config.Rate, int(p.mode.Config.Duration.Seconds())), 2)
	p.PrintLine()
	fmt.Println()
}

func (p *Printer) FarewellMessage() {
	p.PrintTopLine()
	fmt.Println("           ups... we're forgetting all your private keys ;)")
	p.PrintLine()
}

func (p *Printer) SettingFundsMessage() {
	p.Println("", 2)
	if p.mode.autoFundsPrepareEnabled {
		p.Println("Auto funds creation enabled", 1)
	} else {
		p.Println("Auto funds creation disabled", 1)
	}
	p.Println("", 2)
	fmt.Println()
}

func (p *Printer) FundsWarning() {

	p.Println(p.colorString("Not enough fresh faucet outputs in the wallet to spam!", "red"), 2)
	p.PrintlnPoint("Request more manually with 'Prepare faucet funds' option in main menu.", 2)
	p.PrintlnPoint("You can also enable auto funds requesting in the settings.", 2)
	fmt.Println()
}

func (p *Printer) UrlWarning() {
	p.Println(p.colorString("Could not connect to provided API endpoint, client not added.", "red"), 2)
	fmt.Println()

}

func (p *Printer) clients() {
	p.Println(p.colorString("Provided clients:", "cyan"), 1)
	for url := range p.mode.Config.ClientUrls {
		p.PrintlnPoint(url, 2)
	}
}

func (p *Printer) colorString(s string, color string) string {
	colorStringReset := "\033[0m"
	colorString := ""
	switch color {
	case "red":
		colorString = "\033[31m"
	case "cyan":
		colorString = "\033[36m"
	case "green":
		colorString = "\033[32m"
	}

	return colorString + s + colorStringReset
}

func (p *Printer) Settings() {
	p.PrintTopLine()
	p.Println(p.colorString("Current settings:", "cyan"), 0)
	p.Println(fmt.Sprintf("Auto request funds enabled: %v", p.mode.autoFundsPrepareEnabled), 1)
	p.clients()
	p.PrintLine()
	fmt.Println()

}

func (p *Printer) ClientsWarning() {
	p.Println("", 2)
	p.Println(p.colorString("No clients are configured, you can add API urls in the settings.", "red"), 1)
	p.Println("", 2)
	fmt.Println()
}

func (p *Printer) MaxSpamWarning() {
	p.Println("", 2)
	p.Println(p.colorString("Cannot spam. Maximum number of concurrent spams achieved.", "red"), 1)
	p.Println("", 2)
	fmt.Println()
}

func (p *Printer) CurrentSpams() {
	p.mode.spamMutex.Lock()
	defer p.mode.spamMutex.Unlock()

	if len(p.mode.activeSpammers) == 0 {
		p.Println(p.colorString("There are no currently running spams.", "red"), 1)
		return
	}
	p.Println(p.colorString("Currently active spammers:", "green"), 1)
	for id := range p.mode.activeSpammers {
		details := p.mode.spammerLog.SpamDetails(id)
		startTime := p.mode.spammerLog.StartTime(id)
		endTime := startTime.Add(details.Duration)
		timeLeft := int(endTime.Sub(time.Now()).Seconds())
		p.PrintlnPoint(fmt.Sprintf("ID: %d, scenario: %s, time left: %d [s]", id, details.Scenario, timeLeft), 2)
	}
	p.PrintLine()
	fmt.Println()
}

func (p *Printer) History() {
	p.PrintTopLine()
	p.Println(fmt.Sprintf(p.colorString("List of last %d started spams.", "cyan"), lastSpamsShowed), 1)
	p.mode.spammerLog.LogHistory(lastSpamsShowed, os.Stdout)
	p.PrintLine()
	fmt.Println()
}

func (p *Printer) ClientNotFoundWarning(id int) {
	p.Println("", 2)
	p.Println(p.colorString(fmt.Sprintf("No spam with id %d found. Nothing removed.", id), "red"), 1)
	p.Println("", 2)

	fmt.Println()
}

func (p *Printer) NoActiveSpammer() {
	p.Println("", 2)
	p.Println(p.colorString(fmt.Sprintf("No active spammers."), "red"), 1)
	p.Println("", 2)

	fmt.Println()
}

func (p *Printer) FundsCurrentlyPreparedWarning() {
	p.Println("", 2)
	p.Println(p.colorString("Funds are currently prepared. Try again later.", "red"), 1)
	p.Println("", 2)
	fmt.Println()
}

func (p *Printer) StartedPreparingMessage(numToPrepareStr string) {
	p.Println("", 2)
	p.Println(p.colorString("Start preparing "+numToPrepareStr+" faucet outputs.", "green"), 1)
	p.Println("", 2)
	fmt.Println()
}

func (p *Printer) SpammerStartedMessage() {
	p.Println("", 2)
	p.Println(p.colorString("Spammer started", "green"), 1)
	p.Println("", 2)
	fmt.Println()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
