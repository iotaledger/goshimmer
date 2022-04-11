package main

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/workerpool"
	"go.uber.org/atomic"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	faucetFundsCheck    = time.Minute / 4
	queueSize           = 5
	spammingWorkerCount = 5
	minSpamOutputs      = 2000
)

var (
	spammingWorkerPool *workerpool.NonBlockingQueuedWorkerPool
	faucetTicker       *time.Ticker
	printer            *Printer
)

// region action ///////////////////////////////////////////////////////////////////////////////////////////////////////

type action int

const (
	walletDetails action = iota
	prepareFunds
	spam
	settings
	shutdown
)

var actions = []string{"Evil wallet details", "Prepare faucet funds", "Spam", "Settings", "Close"}

var scenarios = []string{"conflict-circle", "guava", "orange", "mango", "pear", "lemon", "banana", "kiwi", "peace"}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

// region interactive ///////////////////////////////////////////////////////////////////////////////////////////////////////

func Run() {
	mode := NewInteractiveMode()

	printer = NewPrinter(mode)

	printer.printBanner()
	time.Sleep(time.Second)
	configure()
	go mode.runBackgroundTasks()
	mode.menu()

	for {
		select {
		case <-mode.mainMenu:
			mode.menu()
		case <-mode.shutdown:
			printer.FarewellMessage()
			os.Exit(0)
			return
		}
	}
}

func configure() {
	spammingWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		fmt.Println("Spamming!")
	}, workerpool.WorkerCount(spammingWorkerCount), workerpool.QueueSize(queueSize))
	faucetTicker = time.NewTicker(faucetFundsCheck)

	//getScenariosNames()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

// region Mode /////////////////////////////////////////////////////////////////////////////////////////////////////////

type Mode struct {
	evilWallet *evilwallet.EvilWallet
	shutdown   chan types.Empty
	mainMenu   chan types.Empty
	action     chan action

	nextAction string
	setting    *settingSurvey

	preparingFunds          bool
	autoFundsPrepareEnabled bool

	msgSent       *atomic.Uint64
	txSent        *atomic.Uint64
	conflictsSent *atomic.Uint64

	stdOutMutex sync.Mutex
}

func NewInteractiveMode() *Mode {
	return &Mode{
		evilWallet: evilwallet.NewEvilWallet(),
		action:     make(chan action),
		shutdown:   make(chan types.Empty),
		mainMenu:   make(chan types.Empty),
		setting:    &settingSurvey{FundsCreation: "enable"},

		msgSent:       atomic.NewUint64(0),
		txSent:        atomic.NewUint64(0),
		conflictsSent: atomic.NewUint64(0),

		autoFundsPrepareEnabled: false,
	}
}

func (m *Mode) runBackgroundTasks() {
	for {
		select {
		case <-faucetTicker.C:
			m.prepareFundsIfNeeded()
		case act := <-m.action:
			switch act {
			case spam:
				m.spamMenu()
			case walletDetails:
				m.walletDetails()
			case prepareFunds:
				m.prepareFunds()
			case settings:
				m.settings()
			case shutdown:
				m.shutdown <- types.Void
			}
			m.mainMenu <- types.Void
		}
	}

}

func (m *Mode) walletDetails() {
	m.stdOutMutex.Lock()
	defer m.stdOutMutex.Unlock()

	printer.EvilWalletStatus()
}

func (m *Mode) menu() {
	m.stdOutMutex.Lock()
	defer m.stdOutMutex.Unlock()
	time.Sleep(time.Second)
	err := survey.AskOne(actionQuestion, &m.nextAction)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	m.onMenuAction()
}

func (m *Mode) onMenuAction() {
	switch m.nextAction {
	case actions[walletDetails]:
		m.action <- walletDetails
	case actions[prepareFunds]:
		m.action <- prepareFunds
	case actions[spam]:
		m.action <- spam
	case actions[settings]:
		m.action <- settings
	case actions[shutdown]:
		m.action <- shutdown
	}

}

func (m *Mode) prepareFundsIfNeeded() {
	if m.evilWallet.UnspentOutputsLeft(evilwallet.Fresh) < minSpamOutputs {
		if !m.preparingFunds && m.autoFundsPrepareEnabled {
			m.preparingFunds = true
			go func() {
				_ = m.evilWallet.RequestFreshBigFaucetWallet()
			}()
			m.preparingFunds = false
		}
	}
}

func (m *Mode) settings() {
	m.stdOutMutex.Lock()
	defer m.stdOutMutex.Unlock()

	err := survey.Ask(settingsQuestion, m.setting)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	m.onSettings()
	printer.SettingFundsMessage()

}

func (m *Mode) onSettings() {
	if m.setting.FundsCreation == "enable" {
		m.autoFundsPrepareEnabled = true
	} else {
		m.autoFundsPrepareEnabled = false
	}
}

func (m *Mode) prepareFunds() {
	m.stdOutMutex.Lock()
	defer m.stdOutMutex.Unlock()

	if m.preparingFunds {
		printer.Println("Funds are currently prepared. Try again later.", 2)
		return
	}
	numToPrepareStr := ""
	err := survey.AskOne(fundsQuestion, &numToPrepareStr)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	switch numToPrepareStr {
	case "100":

		go func() {
			time.Sleep(time.Second)
			m.preparingFunds = true
			_ = m.evilWallet.RequestFreshFaucetWallet()
			m.preparingFunds = false
		}()
	case "10000":
		go func() {
			m.preparingFunds = true
			_ = m.evilWallet.RequestFreshBigFaucetWallet()
			m.preparingFunds = false
		}()
	case "cancel":
		return
	case "50000":
		go func() {
			m.preparingFunds = true
			m.evilWallet.RequestFreshBigFaucetWallets(5)
			m.preparingFunds = false
		}()
	case "100000":
		go func() {
			m.preparingFunds = true
			m.evilWallet.RequestFreshBigFaucetWallets(10)
			m.preparingFunds = false
		}()
	}

	printer.Println("Start preparing "+numToPrepareStr+" faucet outputs.", 2)
}

func (m *Mode) spamMenu() {
	m.stdOutMutex.Lock()
	defer m.stdOutMutex.Unlock()
	time.Sleep(time.Second)
	details := spamDetailsSurvey{}
	err := survey.Ask(spamDetailsQuestions, &details)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	d, _ := strconv.Atoi(details.SpamDuration)
	dur := time.Second * time.Duration(d)
	rate, _ := strconv.Atoi(details.SpamRate)
	s, _ := evilwallet.GetScenario("guava")
	SpamNestedConflicts(m.evilWallet, rate, time.Second, dur, s, true)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
