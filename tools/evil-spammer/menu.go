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
	p.Println("Interactive mode enabled", level1)
	fmt.Println()
}

func (p *Printer) EvilWalletStatus() {
	p.PrintTopLine()
	p.Println(p.colorString("Evil Wallet status:", "cyan"), level2)
	p.PrintlnPoint(fmt.Sprintf("Available faucet outputs: %d", p.mode.evilWallet.UnspentOutputsLeft(evilwallet.Fresh)), level2)
	p.PrintlnPoint(fmt.Sprintf("Available reuse outputs: %d", p.mode.evilWallet.UnspentOutputsLeft(evilwallet.Reuse)), level2)
	p.PrintlnPoint(fmt.Sprintf("Spammed blocks: %d", p.mode.blkSent.Load()), level2)
	p.PrintlnPoint(fmt.Sprintf("Spammed transactions: %d", p.mode.txSent.Load()), level2)
	p.PrintlnPoint(fmt.Sprintf("Spammed scenario batches: %d", p.mode.scenariosSent.Load()), level2)

	p.PrintLine()
	fmt.Println()
}

func (p *Printer) SpammerSettings() {
	rateUnit := "[mpm]"
	if p.mode.Config.timeUnit == time.Second {
		rateUnit = "[mps]"
	}
	p.PrintTopLine()
	p.Println(p.colorString("Current settings:", "cyan"), level1)
	p.PrintlnPoint(fmt.Sprintf("Scenario: %s", p.mode.Config.Scenario), level2)
	p.PrintlnPoint(fmt.Sprintf("Deep: %v, Reuse: %v", p.mode.Config.Deep, p.mode.Config.Reuse), level2)
	p.PrintlnPoint(fmt.Sprintf("Use rate-setter: %v", p.mode.Config.UseRateSetter), level2)
	p.PrintlnPoint(fmt.Sprintf("Rate: %d%s, Duration: %d[s]", p.mode.Config.Rate, rateUnit, int(p.mode.Config.duration.Seconds())), level2)
	p.PrintLine()
	fmt.Println()
}

func (p *Printer) FarewellBlock() {
	p.PrintTopLine()
	fmt.Println("           ups... we're forgetting all your private keys ;)")
	p.PrintLine()
}

func (p *Printer) FundsWarning() {
	p.Println(p.colorString("Not enough fresh faucet outputs in the wallet to spam!", "red"), level1)
	if p.mode.preparingFunds {
		p.PrintlnPoint(p.colorString("Funds are currently prepared, wait until outputs will be available.", "yellow"), level2)
	} else {
		p.PrintlnPoint(p.colorString("Request more outputs manually with 'Prepare faucet funds' option in main menu.", "yellow"), level2)
		p.PrintlnPoint(p.colorString("You can also enable auto funds requesting in the settings.", "yellow"), level2)
	}
	fmt.Println()
}

func (p *Printer) URLWarning() {
	p.Println(p.colorString("Could not connect to provided API endpoint, client not added.", "yellow"), level2)
	fmt.Println()
}

func (p *Printer) URLExists() {
	p.Println(p.colorString("The url already exists.", "red"), level2)
	fmt.Println()
}

func (p *Printer) DevNetFundsWarning() {
	p.Println(p.colorString("Warning: Preparing 10k outputs and more could take looong time in the DevNet due to high PoW and congestion.", "yellow"), level1)
	p.Println(p.colorString("We advice to use 100 option only.", "yellow"), level1)
	fmt.Println()
}

func (p *Printer) NotEnoughClientsWarning(numOfClient int) {
	p.Println(p.colorString(fmt.Sprintf("Warning: At least %d clients is recommended if double spends are not allowed from the same node.", numOfClient), "red"), level2)
	fmt.Println()
}

func (p *Printer) clients() {
	p.Println(p.colorString("Provided clients:", "cyan"), level1)
	for url := range p.mode.Config.clientURLs {
		p.PrintlnPoint(url, level2)
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
	case "yellow":
		colorString = "\033[33m"
	}

	return colorString + s + colorStringReset
}

func (p *Printer) Settings() {
	p.PrintTopLine()
	p.Println(p.colorString("Current settings:", "cyan"), 0)
	p.Println(fmt.Sprintf("Auto requesting enabled: %v", p.mode.Config.AutoRequesting), level1)
	p.Println(fmt.Sprintf("Use rate-setter: %v", p.mode.Config.UseRateSetter), level1)
	p.clients()
	p.PrintLine()
	fmt.Println()
}

func (p *Printer) MaxSpamWarning() {
	p.Println("", level2)
	p.Println(p.colorString("Cannot spam. Maximum number of concurrent spams achieved.", "red"), level1)
	p.Println("", level2)
	fmt.Println()
}

func (p *Printer) CurrentSpams() {
	p.mode.spamMutex.Lock()
	defer p.mode.spamMutex.Unlock()

	lines := make([]string, 0)
	for id := range p.mode.activeSpammers {
		details := p.mode.spammerLog.SpamDetails(id)
		startTime := p.mode.spammerLog.StartTime(id)
		endTime := startTime.Add(details.duration)
		timeLeft := int(time.Until(endTime).Seconds())
		lines = append(lines, fmt.Sprintf("ID: %d, scenario: %s, time left: %d [s]", id, details.Scenario, timeLeft))
	}
	if len(lines) == 0 {
		p.Println(p.colorString("There are no currently running spams.", "red"), level1)
		return
	}
	p.Println(p.colorString("Currently active spammers:", "green"), level1)
	for _, line := range lines {
		p.PrintlnPoint(line, level2)
	}
	p.PrintLine()
	fmt.Println()
}

func (p *Printer) History() {
	p.PrintTopLine()
	p.Println(fmt.Sprintf(p.colorString("List of last %d started spams.", "cyan"), lastSpamsShowed), level1)
	p.mode.spammerLog.LogHistory(lastSpamsShowed, os.Stdout)
	p.PrintLine()
	fmt.Println()
}

func (p *Printer) ClientNotFoundWarning(id int) {
	p.Println("", level2)
	p.Println(p.colorString(fmt.Sprintf("No spam with id %d found. Nothing removed.", id), "red"), level1)
	p.Println("", level2)

	fmt.Println()
}

func (p *Printer) NoActiveSpammer() {
	p.Println("", level2)
	p.Println(p.colorString("No active spammers.", "red"), level1)
	p.Println("", level2)

	fmt.Println()
}

func (p *Printer) FundsCurrentlyPreparedWarning() {
	p.Println("", level2)
	p.Println(p.colorString("Funds are currently prepared. Try again later.", "red"), level1)
	p.Println("", level2)
	fmt.Println()
}

func (p *Printer) StartedPreparingBlock(numToPrepareStr string) {
	p.Println("", level2)
	p.Println(p.colorString("Start preparing "+numToPrepareStr+" faucet outputs.", "green"), level1)
	p.Println("", level2)
	fmt.Println()
}

func (p *Printer) SpammerStartedBlock() {
	p.Println("", level2)
	p.Println(p.colorString("Spammer started", "green"), level1)
	p.Println("", level2)
	fmt.Println()
}

func (p *Printer) AutoRequestingEnabled() {
	p.Println("", level2)
	p.Println(p.colorString(fmt.Sprintf("Automatic funds requesting enabled. %s outputs will be requested whenever output amout will go below %d.", p.mode.Config.AutoRequestingAmount, minSpamOutputs), "green"), level1)
	p.Println(p.colorString("The size of the request can be changed in the config file. Possible values: '100', '10000'", "yellow"), level1)
	p.Println("", level2)
}

func (p *Printer) RateSetterEnabled() {
	p.Println("", level2)
	p.Println(p.colorString("Enable waiting the rate-setter estimate.", "green"), level1)
	p.Println(p.colorString(" Enabling this will force the spammer to sleep certain amount of time based on node's rate-setter estimate.", "yellow"), level1)
	p.Println("", level2)
}

const (
	level1 = 1
	level2 = 2
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
