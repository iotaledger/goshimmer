package main

import (
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/iotaledger/hive.go/logger"
)

var (
	log           = logger.NewLogger("main")
	optionFlagSet = flag.NewFlagSet("script flag set", flag.ExitOnError)
)

func main() {

	parseFlags()

	// run selected test scenario
	switch Script {
	case "basic":
		CustomSpam(&customSpamParams)
	case "quick":
		QuickTest(&quickTest)
	case "increasingDS":
		IncreasingDoubleSpendTest(&increasingDoubleSpendTest)
	case "reasonableDS":
		ReasonableDoubleSpendTest(&reasonableDoubleSpendTest)
	default:
		log.Warnf("Unknown parameter for script, possible values: basic, quick, increasingDS, reasonableDS, verifyLedger")
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
	case "increasingDS":
		parseIncreasingDSFlags()
	case "reasonableDS":
		parseReasonableDSFlags()
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
	spamTypes := optionFlagSet.String("spammers", "", "Spammers used during test. Format: strings separated with comma, available options: 'msg' - message, 'tx' - transaction, 'ds' - double spends spammers")
	rates := optionFlagSet.String("rates", "", "Spamming rates for provided 'spammer'. Format: numbers separated with comma, e.g. 10,100,1 if three spammers were provided for 'spammer' parameter.")
	durations := optionFlagSet.String("durations", "", "Spam duration in seconds. Cannot be combined with flag 'msgNums'. Format: numbers separated with comma, e.g. 10,100,1 if three spammers were provided for 'spammer' parameter.")
	msgNums := optionFlagSet.String("msgNums", "", "Spam duration in seconds. Cannot be combined with flag 'durations'. Format: numbers separated with comma, e.g. 10,100,1 if three spammers were provided for 'spammer' parameter.")
	timeunit := optionFlagSet.Duration("tu", customSpamParams.TimeUnit, "Time unit for the spamming rate. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'.")
	delayBetweenConflicts := optionFlagSet.Duration("dbc", customSpamParams.DelayBetweenConflicts, "delayBetweenConflicts - Time delay between conflicts in double spend spamming")
	verifyLedger := optionFlagSet.Bool("verify", customSpamParams.VerifyLedger, "Set to true if verify ledger script should be run at the end of the test")

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
		log.Info("duration")
		parsedDurations := parseCommaSepInt(*durations)
		customSpamParams.DurationsInSec = parsedDurations
	}
	if *msgNums != "" {
		log.Info("msgnuim")
		parsedMsgNums := parseCommaSepInt(*msgNums)
		customSpamParams.MsgToBeSent = parsedMsgNums
	}
	customSpamParams.TimeUnit = *timeunit
	customSpamParams.DelayBetweenConflicts = *delayBetweenConflicts
	customSpamParams.VerifyLedger = *verifyLedger

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

func parseIncreasingDSFlags() {
	urls := optionFlagSet.String("urls", "", "API urls for clients used in test separated with commas")
	start := optionFlagSet.Int("start", increasingDoubleSpendTest.Start, "The starting rate for the test")
	stop := optionFlagSet.Int("stop", increasingDoubleSpendTest.Stop, "The maximum spamming rate")
	step := optionFlagSet.Int("step", increasingDoubleSpendTest.Step, "Increase of a rate per epoch")
	epochDuration := optionFlagSet.Duration("epochDur", increasingDoubleSpendTest.TimeUnit, "Epoch duration. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'")
	timeUnit := optionFlagSet.Duration("tu", increasingDoubleSpendTest.TimeUnit, "Time unit for the spamming rate. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'.")
	delayBetweenConflicts := optionFlagSet.Duration("dbc", increasingDoubleSpendTest.DelayBetweenConflicts, "Delay Between Conflicts in double spend spamming. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'.")
	verifyLedger := optionFlagSet.Bool("verify", increasingDoubleSpendTest.VerifyLedger, "Set to true if verify ledger script should be run at the end of the test")

	parseOptionFlagSet(optionFlagSet)

	if *urls != "" {
		parsedUrls := parseCommaSepString(*urls)
		increasingDoubleSpendTest.ClientUrls = parsedUrls
	}
	increasingDoubleSpendTest.Start = *start
	increasingDoubleSpendTest.Stop = *stop
	increasingDoubleSpendTest.Step = *step
	increasingDoubleSpendTest.EpochDuration = *epochDuration
	increasingDoubleSpendTest.TimeUnit = *timeUnit
	increasingDoubleSpendTest.DelayBetweenConflicts = *delayBetweenConflicts
	increasingDoubleSpendTest.VerifyLedger = *verifyLedger
}

func parseReasonableDSFlags() {
	urls := optionFlagSet.String("urls", "", "API urls for clients used in test separated with commas")
	dataRate := optionFlagSet.Int("dataRate", reasonableDoubleSpendTest.DataRate, "The data spamming rate")
	dsRate := optionFlagSet.Int("dsRate", reasonableDoubleSpendTest.DsRate, "The double spend spamming rate")
	timeunit := optionFlagSet.Duration("tu", reasonableDoubleSpendTest.TimeUnit, "Time unit for the spamming rate. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'")
	epochDuration := optionFlagSet.Duration("epochDur", reasonableDoubleSpendTest.EpochDuration, "Epoch duration. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'")
	epochsNumber := optionFlagSet.Int("epochNum", reasonableDoubleSpendTest.EpochsNumber, "The number of epochs, equals to how many times spam will be repeated")
	delayBetweenConflicts := optionFlagSet.Duration("dbc", reasonableDoubleSpendTest.DelayBetweenConflicts, "Delay Between Conflicts in double spend spamming. Format: decimal numbers, each with optional fraction and a unit suffix, such as '300ms', '-1.5h' or '2h45m'.\n Valid time units are 'ns', 'us', 'ms', 's', 'm', 'h'.")
	verifyLedger := optionFlagSet.Bool("verify", reasonableDoubleSpendTest.VerifyLedger, "Set to true if verify ledger script should be run at the end of the test")

	parseOptionFlagSet(optionFlagSet)

	if *urls != "" {
		parsedUrls := parseCommaSepString(*urls)
		reasonableDoubleSpendTest.ClientUrls = parsedUrls
	}
	reasonableDoubleSpendTest.DataRate = *dataRate
	reasonableDoubleSpendTest.DsRate = *dsRate
	reasonableDoubleSpendTest.TimeUnit = *timeunit
	reasonableDoubleSpendTest.EpochDuration = *epochDuration
	reasonableDoubleSpendTest.EpochsNumber = *epochsNumber
	reasonableDoubleSpendTest.DelayBetweenConflicts = *delayBetweenConflicts
	reasonableDoubleSpendTest.VerifyLedger = *verifyLedger
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
