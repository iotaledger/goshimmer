package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/iotaledger/goshimmer/client"
	"github.com/iotaledger/goshimmer/client/evilspammer"
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/hive.go/types"
	"go.uber.org/atomic"
)

const (
	faucetFundsCheck   = time.Minute / 4
	maxConcurrentSpams = 5
	minSpamOutputs     = 2000
	lastSpamsShowed    = 15
	timeFormat         = "2006/01/02 15:04:05"
)

var (
	faucetTicker *time.Ticker
	printer      *Printer
)

type InteractiveConfig struct {
	WebAPI      []string `json:"webAPI"`
	Rate        int      `json:"rate"`
	DurationStr string   `json:"duration"`
	TimeUnitStr string   `json:"timeUnit"`
	Deep        bool     `json:"deepEnabled"`
	Reuse       bool     `json:"reuseEnabled"`
	Scenario    string   `json:"scenario"`

	duration   time.Duration
	timeUnit   time.Duration
	clientUrls map[string]types.Empty
}

var configJSON = `{
	"webAPI": ["http://127.0.0.1:8080","http://127.0.0.1:8090"],
	"rate": 2,
	"duration": "20s",
	"timeUnit": "1s",
	"deepEnabled": false,
	"reuseEnabled": true,
	"scenario": "tx"
}`

// region survey selections  ///////////////////////////////////////////////////////////////////////////////////////////////////////

type action int

const (
	actionWalletDetails action = iota
	actionPrepareFunds
	actionSpamMenu
	actionCurrent
	actionHistory
	actionSettings
	shutdown
)

var actions = []string{"Evil wallet details", "Prepare faucet funds", "New spam", "Currently running", "Spam history", "Settings", "Close"}

const (
	spamScenario = "Change scenario"
	spamType     = "Update spam options"
	spamDetails  = "Update spam rate and duration"
	startSpam    = "Start the spammer"
	back         = "Go back"
)

var spamMenuOptions = []string{startSpam, spamScenario, spamDetails, spamType, back}

const (
	settingPreparation = "Auto funds requesting"
	settingAddUrls     = "Add client API url"
	settingRemoveUrls  = "Remove client API urls"
)

var settingsMenuOptions = []string{settingPreparation, settingAddUrls, settingRemoveUrls, back}

const (
	currentSpamRemove = "Cancel spam"
)

var currentSpamOptions = []string{currentSpamRemove, back}

var (
	scenarios     = []string{"msg", "tx", "ds", "conflict-circle", "guava", "orange", "mango", "pear", "lemon", "banana", "kiwi", "peace"}
	confirms      = []string{"enable", "disable"}
	outputNumbers = []string{"100", "10000", "50000", "100000", "cancel"}
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

// region interactive ///////////////////////////////////////////////////////////////////////////////////////////////////////

func Run() {
	mode := NewInteractiveMode()

	printer = NewPrinter(mode)

	printer.printBanner()
	mode.loadConfig()
	time.Sleep(time.Second)
	configure()
	go mode.runBackgroundTasks()
	mode.menu()

	for {
		select {
		case id := <-mode.spamFinished:
			mode.summarizeSpam(id)
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
	faucetTicker = time.NewTicker(faucetFundsCheck)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////

// region Mode /////////////////////////////////////////////////////////////////////////////////////////////////////////

type Mode struct {
	evilWallet   *evilwallet.EvilWallet
	shutdown     chan types.Empty
	mainMenu     chan types.Empty
	spamFinished chan int
	action       chan action

	nextAction string

	preparingFunds          bool
	autoFundsPrepareEnabled bool

	Config        InteractiveConfig
	msgSent       *atomic.Uint64
	txSent        *atomic.Uint64
	scenariosSent *atomic.Uint64

	activeSpammers map[int]*evilspammer.Spammer
	spammerLog     *SpammerLog
	spamMutex      sync.Mutex

	stdOutMutex sync.Mutex
}

func NewInteractiveMode() *Mode {
	return &Mode{
		evilWallet:   evilwallet.NewEvilWallet(),
		action:       make(chan action),
		shutdown:     make(chan types.Empty),
		mainMenu:     make(chan types.Empty),
		spamFinished: make(chan int),

		Config:        InteractiveConfig{clientUrls: make(map[string]types.Empty)},
		msgSent:       atomic.NewUint64(0),
		txSent:        atomic.NewUint64(0),
		scenariosSent: atomic.NewUint64(0),

		spammerLog:              NewSpammerLog(),
		activeSpammers:          make(map[int]*evilspammer.Spammer),
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
			case actionHistory:
				m.history()
				m.mainMenu <- types.Void
			case actionCurrent:
				go m.currentSpams()
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
	case actions[actionCurrent]:
		m.action <- actionCurrent
	case actions[actionHistory]:
		m.action <- actionHistory
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
		printer.FundsCurrentlyPreparedWarning()
		return
	}
	if len(m.Config.clientUrls) == 0 {
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

	printer.StartedPreparingMessage(numToPrepareStr)
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
}

func (m *Mode) spamSubMenu(menuType string) {
	switch menuType {
	case spamDetails:
		var spamSurvey spamDetailsSurvey
		err := survey.Ask(spamDetailsQuestions(strconv.Itoa(int(m.Config.duration.Seconds())), strconv.Itoa(m.Config.Rate)), &spamSurvey)
		if err != nil {
			fmt.Println(err.Error())
			m.mainMenu <- types.Void
			return
		}
		m.parseSpamDetails(spamSurvey)

	case spamType:
		var spamSurvey spamTypeSurvey
		err := survey.Ask(spamTypeQuestions(boolToEnable(m.Config.Deep), boolToEnable(m.Config.Reuse)), &spamSurvey)
		if err != nil {
			fmt.Println(err.Error())
			m.mainMenu <- types.Void
			return
		}
		m.parseSpamType(spamSurvey)

	case spamScenario:
		scenario := ""
		err := survey.AskOne(spamScenarioQuestion(m.Config.Scenario), &scenario)
		if err != nil {
			fmt.Println(err.Error())
			m.mainMenu <- types.Void
			return
		}
		m.parseScenario(scenario)

	case startSpam:
		if m.areEnoughFundsAvailable() {
			printer.FundsWarning()
			m.mainMenu <- types.Void
			return
		}
		if len(m.Config.clientUrls) == 0 {
			printer.ClientsWarning()
			m.mainMenu <- types.Void
			return
		}
		if len(m.activeSpammers) >= maxConcurrentSpams {
			printer.MaxSpamWarning()
			m.mainMenu <- types.Void
			return
		}
		m.startSpam()

	case back:
		m.mainMenu <- types.Void
		return
	}
	m.action <- actionSpamMenu
}

func (m *Mode) areEnoughFundsAvailable() bool {
	return m.evilWallet.UnspentOutputsLeft(evilwallet.Fresh) < m.Config.Rate*int(m.Config.duration.Seconds()) && m.Config.Scenario != "msg"
}

func (m *Mode) startSpam() {
	m.spamMutex.Lock()
	defer m.spamMutex.Unlock()

	s, _ := evilwallet.GetScenario(m.Config.Scenario)
	var spammer *evilspammer.Spammer
	spamId := m.spammerLog.AddSpam(m.Config)
	spammer = SpamNestedConflicts(m.evilWallet, m.Config.Rate, time.Second, m.Config.duration, s, m.Config.Deep, m.Config.Reuse)
	m.activeSpammers[spamId] = spammer
	go func(id int) {
		spammer.Spam()
		m.spamFinished <- id
	}(spamId)
	printer.SpammerStartedMessage()
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
		m.Config.clientUrls[url] = types.Void
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

func (m *Mode) history() {
	m.stdOutMutex.Lock()
	defer m.stdOutMutex.Unlock()
	printer.History()
}

func (m *Mode) currentSpams() {
	m.stdOutMutex.Lock()
	defer m.stdOutMutex.Unlock()

	printer.CurrentSpams()
	answer := ""
	err := survey.AskOne(currentMenuQuestion, &answer)
	if err != nil {
		fmt.Println(err.Error())
		m.mainMenu <- types.Void
		return
	}

	m.currentSpamsSubMenu(answer)
}

func (m *Mode) currentSpamsSubMenu(menuType string) {
	switch menuType {
	case currentSpamRemove:
		if len(m.activeSpammers) == 0 {
			printer.NoActiveSpammer()
		} else {
			answer := ""
			err := survey.AskOne(removeSpammer, &answer)
			if err != nil {
				fmt.Println(err.Error())
				m.mainMenu <- types.Void
				return
			}
			m.parseIdToRemove(answer)
		}

		m.action <- actionCurrent

	case back:
		m.mainMenu <- types.Void
		return
	}
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
	m.Config.duration = dur
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
		if _, ok := m.Config.clientUrls[url]; ok {
			delete(m.Config.clientUrls, url)
			m.evilWallet.RemoveClient(url)
		}
	}
}

func (m *Mode) urlMapToList() (list []string) {
	for url := range m.Config.clientUrls {
		list = append(list, url)
	}
	return
}

func (m *Mode) parseIdToRemove(answer string) {
	m.spamMutex.Lock()
	defer m.spamMutex.Unlock()

	id, err := strconv.Atoi(answer)
	if err != nil {
		return
	}
	m.summarizeSpam(id)

}

func (m *Mode) summarizeSpam(id int) {
	if s, ok := m.activeSpammers[id]; ok {
		m.updateSentStatistic(s, id)
		m.spammerLog.SetSpamEndTime(id)
		delete(m.activeSpammers, id)
	} else {
		printer.ClientNotFoundWarning(id)
	}
}

func (m *Mode) updateSentStatistic(spammer *evilspammer.Spammer, id int) {
	msgSent := spammer.MessagesSent()
	scenariosCreated := spammer.BatchesPrepared()
	if m.spammerLog.SpamDetails(id).Scenario == "msg" {
		m.msgSent.Add(msgSent)
	} else {
		m.txSent.Add(msgSent)
	}
	m.scenariosSent.Add(scenariosCreated)
}

// load the config file
func (m *Mode) loadConfig() {
	path := "tools/evil-spammer/"
	// open config file
	file, err := os.Open(path + "config.json")
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}

		if err = os.WriteFile(path+"config.json", []byte(configJSON), 0o644); err != nil {
			panic(err)
		}
		if file, err = os.Open(path + "config.json"); err != nil {
			panic(err)
		}
	}
	defer file.Close()

	// decode config file
	if err = json.NewDecoder(file).Decode(&m.Config); err != nil {
		panic(err)
	}
	// convert urls array to map
	for _, url := range m.Config.WebAPI {
		m.Config.clientUrls[url] = types.Void
	}
	// parse duration
	d, err := time.ParseDuration(m.Config.DurationStr)
	if err != nil {
		d = time.Minute
	}
	u, err := time.ParseDuration(m.Config.TimeUnitStr)
	if err != nil {
		u = time.Second
	}
	m.Config.duration = d
	m.Config.timeUnit = u
}

func enableToBool(e string) bool {
	return e == "enable"
}

func boolToEnable(b bool) string {
	if b {
		return "enable"
	}
	return "disable"
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

// region SpammerLog ///////////////////////////////////////////////////////////////////////////////////////////////////////////

var historyHeader = "scenario\tstart\tstop\tdeep\treuse\trate\tduration"
var historyLineFmt = "%s\t%s\t%s\t%v\t%v\t%d\t%d\n"

type SpammerLog struct {
	spamDetails   []InteractiveConfig
	spamStartTime []time.Time
	spamStopTime  []time.Time
	tabWriter     io.Writer
	mu            sync.Mutex
}

func NewSpammerLog() *SpammerLog {
	return &SpammerLog{
		spamDetails:   make([]InteractiveConfig, 0),
		spamStartTime: make([]time.Time, 0),
		spamStopTime:  make([]time.Time, 0),
	}
}

func (s *SpammerLog) SpamDetails(spamId int) *InteractiveConfig {
	return &s.spamDetails[spamId]
}

func (s *SpammerLog) StartTime(spamId int) time.Time {
	return s.spamStartTime[spamId]
}

func (s *SpammerLog) AddSpam(config InteractiveConfig) (spamId int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.spamDetails = append(s.spamDetails, config)
	s.spamStartTime = append(s.spamStartTime, time.Now())
	s.spamStopTime = append(s.spamStopTime, time.Time{})
	return len(s.spamDetails) - 1
}

func (s *SpammerLog) SetSpamEndTime(spamId int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.spamStopTime[spamId] = time.Now()
}

func newTabWriter(writer io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(writer, 0, 0, 1, ' ', tabwriter.Debug|tabwriter.TabIndent)
}

func (s *SpammerLog) LogHistory(lastLines int, writer io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w := newTabWriter(writer)
	fmt.Fprintln(w, historyHeader)
	idx := len(s.spamDetails) - lastLines + 1
	if idx < 0 {
		idx = 0
	}
	for i, spam := range s.spamDetails[idx:] {
		fmt.Fprintf(w, historyLineFmt, spam.Scenario, s.spamStartTime[i].Format(timeFormat), s.spamStopTime[i].Format(timeFormat),
			spam.Deep, spam.Deep, spam.Rate, int(spam.duration.Seconds()))
	}
	w.Flush()
	return
}

func (s *SpammerLog) LogSelected(lines []int, writer io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w := newTabWriter(writer)
	fmt.Fprintln(w, historyHeader)
	for _, idx := range lines {
		spam := s.spamDetails[idx]
		fmt.Fprintf(w, historyLineFmt, spam.Scenario, s.spamStartTime[idx].Format(timeFormat), s.spamStopTime[idx].Format(timeFormat),
			spam.Deep, spam.Deep, spam.Rate, int(spam.duration.Seconds()))
	}
	w.Flush()
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
