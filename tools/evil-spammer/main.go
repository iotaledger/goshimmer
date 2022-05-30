package main

import (
	"flag"
	"fmt"
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/tools/evil-spammer/evillogger"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	log           = evillogger.New("main")
	optionFlagSet = flag.NewFlagSet("script flag set", flag.ExitOnError)
)

func main() {
	help := parseFlags()
	if help {
		fmt.Println("Usage of the Evil Spammer tool, provide the first argument for the selected mode:\n" +
			"'interactive' - enters the interactive mode.\n" +
			"'basic' - can be parametrized with additional flags to run one time spammer. Run 'evil-wallet basic -h' for the list of possible flags.\n" +
			"'quick' - runs simple stress test: tx spam -> msg spam -> ds spam. Run 'evil-wallet quick -h' for the list of possible flags.")
		return
	}
	// run selected test scenario
	switch Script {
	case "interactive":
		Run()
	case "basic":
		CustomSpam(&customSpamParams)
	case "quick":
		QuickTest(&quickTest)
	default:
		log.Warnf("Unknown parameter for script, possible values: basic, quick, interactive")
	}
}

func parseFlags() (help bool) {
	if len(os.Args) <= 1 {
		return true
	}
	script := os.Args[1]

	Script = script
	log.Infof("script %s", Script)

	switch Script {
	case "basic":
		parseBasicSpamFlags()
	case "quick":
		parseQuickTestFlags()
	}
	if Script == "help" || Script == "-h" || Script == "--help" {
		return true
	}
	return
}

func parseOptionFlagSet(flagSet *flag.FlagSet) {
	err := flagSet.Parse(os.Args[2:])
	if err != nil {
		log.Errorf("Cannot parse first `script` parameter")
		return
	}
}

func parseBasicSpamFlags() {
	urls := optionFlagSet.String("urls", "", "API urls for clients used in test separated with commas")
	spamTypes := optionFlagSet.String("spammer", "", "Spammers used during test. Format: strings separated with comma, available options: 'msg' - message,"+
		" 'tx' - transaction, 'ds' - double spends spammers, 'nds' - n-spends spammer, 'custom' - spams with provided scenario")
	rate := optionFlagSet.String("rate", "", "Spamming rate for provided 'spammer'. Format: numbers separated with comma, e.g. 10,100,1 if three spammers were provided for 'spammer' parameter.")
	duration := optionFlagSet.String("duration", "", "Spam duration. Cannot be combined with flag 'msgNum'. Format: separated by commas list of decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'.")
	msgNum := optionFlagSet.String("msgNum", "", "Spam duration in seconds. Cannot be combined with flag 'duration'. Format: numbers separated with comma, e.g. 10,100,1 if three spammers were provided for 'spammer' parameter.")
	timeunit := optionFlagSet.Duration("tu", customSpamParams.TimeUnit, "Time unit for the spamming rate. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'.")
	delayBetweenConflicts := optionFlagSet.Duration("dbc", customSpamParams.DelayBetweenConflicts, "delayBetweenConflicts - Time delay between conflicts in double spend spamming")
	scenario := optionFlagSet.String("scenario", "", "Name of the EvilBatch that should be used for the spam. By default uses Scenario1. Possible scenarios can be found in evilwallet/customscenarion.go.")
	deepSpam := optionFlagSet.Bool("deep", customSpamParams.DeepSpam, "Enable the deep spam, by reusing outputs created during the spam.")

	parseOptionFlagSet(optionFlagSet)

	if *urls != "" {
		parsedUrls := parseCommaSepString(*urls)
		quickTest.ClientUrls = parsedUrls
	}
	if *spamTypes != "" {
		parsedSpamTypes := parseCommaSepString(*spamTypes)
		customSpamParams.SpamTypes = parsedSpamTypes
	}
	if *rate != "" {
		parsedRates := parseCommaSepInt(*rate)
		customSpamParams.Rates = parsedRates
	}
	if *duration != "" {
		parsedDurations := parseDurations(*duration)
		customSpamParams.Durations = parsedDurations
	}
	if *msgNum != "" {
		parsedMsgNums := parseCommaSepInt(*msgNum)
		customSpamParams.MsgToBeSent = parsedMsgNums
	}
	if *scenario != "" {
		conflictBatch, ok := evilwallet.GetScenario(*scenario)
		if ok {
			customSpamParams.Scenario = conflictBatch
		}
	}
	customSpamParams.DeepSpam = *deepSpam
	customSpamParams.TimeUnit = *timeunit
	customSpamParams.DelayBetweenConflicts = *delayBetweenConflicts

	// fill in unused parameter: msgNum or duration with zeros
	if *duration == "" && *msgNum != "" {
		customSpamParams.Durations = make([]time.Duration, len(customSpamParams.MsgToBeSent))
	}
	if *msgNum == "" && *duration != "" {
		customSpamParams.MsgToBeSent = make([]int, len(customSpamParams.Durations))
	}
}

func parseQuickTestFlags() {
	urls := optionFlagSet.String("urls", "", "API urls for clients used in test separated with commas")
	rate := optionFlagSet.Int("rate", quickTest.Rate, "The spamming rate")
	duration := optionFlagSet.Duration("duration", quickTest.Duration, "Duration of the spam. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'.")
	timeunit := optionFlagSet.Duration("tu", quickTest.TimeUnit, "Time unit for the spamming rate. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'.")
	delayBetweenConflicts := optionFlagSet.Duration("dbc", quickTest.DelayBetweenConflicts, "delayBetweenConflicts - Time delay between conflicts in double spend spamming")
	verifyLedger := optionFlagSet.Bool("verify", quickTest.VerifyLedger, "Set to true if verify ledger script should be run at the end of the test")

	parseOptionFlagSet(optionFlagSet)

	if *urls != "" {
		parsedUrls := parseCommaSepString(*urls)
		quickTest.ClientUrls = parsedUrls
	}
	quickTest.Rate = *rate
	quickTest.Duration = *duration
	quickTest.TimeUnit = *timeunit
	quickTest.DelayBetweenConflicts = *delayBetweenConflicts
	quickTest.VerifyLedger = *verifyLedger
}

func parseCommaSepString(urls string) []string {
	split := strings.Split(urls, ",")
	return split
}

func parseCommaSepInt(nums string) []int {
	split := strings.Split(nums, ",")
	parsed := make([]int, len(split))
	for i, num := range split {
		parsed[i], _ = strconv.Atoi(num)
	}
	return parsed
}

func parseDurations(durations string) []time.Duration {
	split := strings.Split(durations, ",")
	parsed := make([]time.Duration, len(split))
	for i, dur := range split {
		parsed[i], _ = time.ParseDuration(dur)
	}
	return parsed
}
