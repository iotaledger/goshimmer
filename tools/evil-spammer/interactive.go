package main

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/iotaledger/goshimmer/client"
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

type InteractiveConfig struct {
	ClientUrls map[string]types.Empty
	Rate       int
	Duration   time.Duration
	TimeUnit   time.Duration
	Deep       bool
	Reuse      bool
	Scenario   string
}

// region survey selections  ///////////////////////////////////////////////////////////////////////////////////////////////////////

type action int

const (
	actionWalletDetails action = iota
	actionPrepareFunds
	actionSpamMenu
	actionSettings
	shutdown
)

var actions = []string{"Evil wallet details", "Prepare faucet funds", "Spam", "Settings", "Close"}

const (
	spamScenario = "Change scenario"
	spamType     = "Update spam options"
	spamDetails  = "Update spam rate and duration"
	startSpam    = "Start the spammer"
	back         = "Go back"
)

var spamMenuOptions = []string{spamScenario, spamType, spamDetails, startSpam, back}

const (
	settingPreparation = "Auto funds requesting"
	settingAddUrls     = "Add client API url"
	settingRemoveUrls  = "Remove client API urls"
)

var settingsMenuOptions = []string{settingPreparation, settingAddUrls, settingRemoveUrls, back}

var (
	scenarios     = []string{"tx", "ds", "conflict-circle", "guava", "orange", "mango", "pear", "lemon", "banana", "kiwi", "peace"}
	confirms      = []string{"enable", "disable"}
	outputNumbers = []string{"100", "10000", "50000", "100000", "cancel"}
)

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

	preparingFunds          bool
	autoFundsPrepareEnabled bool

	Config        InteractiveConfig
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

		Config:        interactive,
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
			case actionSpamMenu:
				go m.spamMenu()
			case actionWalletDetails:
				m.walletDetails()
				m.mainMenu <- types.Void
			case actionPrepareFunds:
				m.prepareFunds()
				m.mainMenu <- types.Void
			case actionSettings:
				go m.settingsMenu()
			case shutdown:
				m.shutdown <- types.Void
			}
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
	err := survey.AskOne(actionQuestion, &m.nextAction)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	m.onMenuAction()
}

func (m *Mode) onMenuAction() {
	switch m.nextAction {
	case actions[actionWalletDetails]:
		m.action <- actionWalletDetails
	case actions[actionPrepareFunds]:
		m.action <- actionPrepareFunds
	case actions[actionSpamMenu]:
		m.action <- actionSpamMenu
	case actions[actionSettings]:
		m.action <- actionSettings
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

func (m *Mode) onSettings() {

}

func (m *Mode) prepareFunds() {
	m.stdOutMutex.Lock()
	defer m.stdOutMutex.Unlock()

	if m.preparingFunds {
		printer.Println("Funds are currently prepared. Try again later.", 2)
		return
	}
	if len(m.Config.ClientUrls) == 0 {
		printer.ClientsWarning()
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
	printer.SpammerSettings()
	var submenu string
	err := survey.AskOne(spamMenuQuestion, &submenu)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	m.spamSubMenu(submenu)

	//d, _ := strconv.Atoi(details.SpamDuration)
	//dur := time.Second * time.Duration(d)
	//rate, _ := strconv.Atoi(details.SpamRate)
	//s, _ := evilwallet.GetScenario("guava")
	//SpamNestedConflicts(m.evilWallet, rate, time.Second, dur, s, true)
}

func (m *Mode) spamSubMenu(menuType string) {
	switch menuType {
	case spamDetails:
		var spamSurvey spamDetailsSurvey
		err := survey.Ask(spamDetailsQuestions, &spamSurvey)
		if err != nil {
			fmt.Println(err.Error())
			m.mainMenu <- types.Void
			return
		}
		m.parseSpamDetails(spamSurvey)

	case spamType:
		var spamSurvey spamTypeSurvey
		err := survey.Ask(spamTypeQuestions, &spamSurvey)
		if err != nil {
			fmt.Println(err.Error())
			m.mainMenu <- types.Void
			return
		}
		m.parseSpamType(spamSurvey)

	case spamScenario:
		scenario := ""
		err := survey.AskOne(spamScenarioQuestion, &scenario)
		if err != nil {
			fmt.Println(err.Error())
			m.mainMenu <- types.Void
			return
		}
		m.parseScenario(scenario)

	case startSpam:
		if m.evilWallet.UnspentOutputsLeft(evilwallet.Fresh) < m.Config.Rate*int(m.Config.Duration.Seconds()) {
			printer.FundsWarning()
			m.mainMenu <- types.Void
			return
		}
		if len(m.Config.ClientUrls) == 0 {
			printer.ClientsWarning()
			m.mainMenu <- types.Void
			return
		}
		s, _ := evilwallet.GetScenario(m.Config.Scenario)
		go SpamNestedConflicts(m.evilWallet, m.Config.Rate, time.Second, m.Config.Duration, s, true)
		printer.Println("Spammer started", 3)

	case back:
		m.mainMenu <- types.Void
		return
	}
	m.action <- actionSpamMenu
}

func (m *Mode) settingsMenu() {
	m.stdOutMutex.Lock()
	defer m.stdOutMutex.Unlock()
	printer.Settings()
	var submenu string
	err := survey.AskOne(settingsQuestion, &submenu)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	m.settingsSubMenu(submenu)
}

func (m *Mode) settingsSubMenu(menuType string) {
	switch menuType {
	case settingPreparation:
		answer := ""
		err := survey.AskOne(autoCreationQuestion, &answer)
		if err != nil {
			fmt.Println(err.Error())
			m.mainMenu <- types.Void
			return
		}
		m.onFundsCreation(answer)

	case settingAddUrls:
		var url string
		err := survey.AskOne(addUrlQuestion, &url)
		if err != nil {
			fmt.Println(err.Error())
			m.mainMenu <- types.Void
			return
		}
		m.validateAndAddUrl(url)

	case settingRemoveUrls:
		answer := make([]string, 0)
		urlsList := m.urlMapToList()
		err := survey.AskOne(removeUrlQuestion(urlsList), &answer)
		if err != nil {
			fmt.Println(err.Error())
			m.mainMenu <- types.Void
			return
		}
		m.removeUrls(answer)

	case back:
		m.mainMenu <- types.Void
		return
	}
	m.action <- actionSettings
}

func (m *Mode) validateAndAddUrl(url string) {
	url = "http://" + url
	ok := validateUrl(url)
	if !ok {
		printer.UrlWarning()
	} else {
		m.Config.ClientUrls[url] = types.Void
		m.evilWallet.AddClient(url)
	}
}

func (m *Mode) onFundsCreation(answer string) {
	if answer == "enable" {
		m.autoFundsPrepareEnabled = true
	} else {
		m.autoFundsPrepareEnabled = false
	}
}

func (m *Mode) settings() {
	m.stdOutMutex.Lock()
	defer m.stdOutMutex.Unlock()

	answer := ""
	err := survey.AskOne(settingsQuestion, &answer)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	m.onSettings()
	printer.SettingFundsMessage()

}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region parsers /////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (m *Mode) parseSpamDetails(details spamDetailsSurvey) {
	d, _ := strconv.Atoi(details.SpamDuration)
	dur := time.Second * time.Duration(d)
	rate, err := strconv.Atoi(details.SpamRate)
	if err != nil {
		return
	}
	m.Config.Rate = rate
	m.Config.Duration = dur
}

func (m *Mode) parseSpamType(spamType spamTypeSurvey) {
	deep := enableToBool(spamType.DeepSpamEnabled)
	reuse := enableToBool(spamType.ReuseLaterEnabled)
	m.Config.Deep = deep
	m.Config.Reuse = reuse
}

func (m *Mode) parseScenario(scenario string) {
	m.Config.Scenario = scenario
}

func (m *Mode) removeUrls(urls []string) {
	for _, url := range urls {
		if _, ok := m.Config.ClientUrls[url]; ok {
			delete(m.Config.ClientUrls, url)
			m.evilWallet.RemoveClient(url)
		}
	}
}

func (m *Mode) urlMapToList() (list []string) {
	for url := range m.Config.ClientUrls {
		list = append(list, url)
	}
	return
}

func enableToBool(e string) bool {
	return e == "enable"
}

func validateUrl(url string) (ok bool) {
	clt := client.NewGoShimmerAPI(url)
	_, err := clt.Info()
	if err != nil {
		return
	}
	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
