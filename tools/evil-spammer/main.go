package main

import (
	"flag"
	"github.com/iotaledger/goshimmer/client/evilwallet"
	"github.com/iotaledger/goshimmer/tools/evil-spammer/evillogger"
	"github.com/iotaledger/goshimmer/tools/evil-spammer/interactive"
	"os"
	"strconv"
	"strings"
)

var (
	log           = evillogger.New("main")
	optionFlagSet = flag.NewFlagSet("script flag set", flag.ExitOnError)
)

func main() {
	parseFlags()

	// run selected test scenario
	switch Script {
	case "interactive":
		interactive.Run()
	case "basic":
		CustomSpam(&customSpamParams)
	case "quick":
		QuickTest(&quickTest)
	default:
		log.Warnf("Unknown parameter for script, possible values: basic, quick, interactive")
	}
}

func parseFlags() {
	script := os.Args[1]

	Script = script
	log.Infof("script %s", Script)

	switch Script {
	case "basic":
		parseBasicSpamFlags()
	case "quick":
		parseQuickTestFlags()
	}

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
	spamTypes := optionFlagSet.String("spammers", "", "Spammers used during test. Format: strings separated with comma, available options: 'msg' - message,"+
		" 'tx' - transaction, 'ds' - double spends spammers, 'nds' - n-spends spammer, 'custom' - spams with provided scenario")
	rates := optionFlagSet.String("rates", "", "Spamming rates for provided 'spammer'. Format: numbers separated with comma, e.g. 10,100,1 if three spammers were provided for 'spammer' parameter.")
	durations := optionFlagSet.String("durations", "", "Spam duration in seconds. Cannot be combined with flag 'msgNums'. Format: numbers separated with comma, e.g. 10,100,1 if three spammers were provided for 'spammer' parameter.")
	msgNums := optionFlagSet.String("msgNums", "", "Spam duration in seconds. Cannot be combined with flag 'durations'. Format: numbers separated with comma, e.g. 10,100,1 if three spammers were provided for 'spammer' parameter.")
	timeunit := optionFlagSet.Duration("tu", customSpamParams.TimeUnit, "Time unit for the spamming rate. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'.")
	delayBetweenConflicts := optionFlagSet.Duration("dbc", customSpamParams.DelayBetweenConflicts, "delayBetweenConflicts - Time delay between conflicts in double spend spamming")
	scenario := optionFlagSet.String("scenario", "", "Name of the EvilBatch that should be used for the spam. By default uses Scenario1. Possible scenarions can be found in evilwallet/customscenarion.go.")
	deepSpam := optionFlagSet.Bool("deep", false, "Enable the deep spam, by reusing outputs created during the spam.")

	parseOptionFlagSet(optionFlagSet)

	// only one of parameters: msgNums or durations can be accepted
	if *durations != "" {
		*msgNums = ""
	}

	if *urls != "" {
		parsedUrls := parseCommaSepString(*urls)
		quickTest.ClientUrls = parsedUrls
	}
	if *spamTypes != "" {
		parsedSpamTypes := parseCommaSepString(*spamTypes)
		customSpamParams.SpamTypes = parsedSpamTypes
	}
	if *rates != "" {
		parsedRates := parseCommaSepInt(*rates)
		customSpamParams.Rates = parsedRates
	}
	if *durations != "" {
		parsedDurations := parseCommaSepInt(*durations)
		customSpamParams.DurationsInSec = parsedDurations
	}
	if *msgNums != "" {
		parsedMsgNums := parseCommaSepInt(*msgNums)
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

	// fill in unused parameter: msgNums or durations with zeros
	if *durations == "" {
		customSpamParams.DurationsInSec = make([]int, len(customSpamParams.MsgToBeSent))
	}
	if *msgNums == "" {
		customSpamParams.MsgToBeSent = make([]int, len(customSpamParams.DurationsInSec))
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
