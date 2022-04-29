package main

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/cockroachdb/errors"
	"strconv"
)

// region survey  //////////////////////////////////////////////////////////////////////////////////////////////

var actionQuestion = &survey.Select{
	Message: "Choose an action",
	Options: actions,
	Default: "Evil wallet details",
}

var fundsQuestion = &survey.Select{
	Message: "How many fresh outputs you want to create?",
	Options: outputNumbers,
	Default: "100",
}
var settingsQuestion = &survey.Select{
	Message: "Available settings:",
	Options: settingsMenuOptions,
	Default: settingPreparation,
}

var autoCreationQuestion = &survey.Select{
	Message: "Enable automatic faucet output creation",
	Options: confirms,
	Default: "enable",
}

var addUrlQuestion = &survey.Input{
	Message: "http://",
	Default: "enable",
}

var removeUrlQuestion = func(urls []string) *survey.MultiSelect {
	return &survey.MultiSelect{
		Message: "Select urls that should be removed.",
		Options: urls,
	}
}

type spamTypeSurvey struct {
	DeepSpamEnabled   string
	ReuseLaterEnabled string
}

var spamTypeQuestions = func(defaultDeep, defaultReuse string) []*survey.Question {
	return []*survey.Question{
		{
			Name: "deepSpamEnabled",
			Prompt: &survey.Select{
				Message: "Deep spam",
				Options: confirms,
				Default: defaultDeep,
				Help:    "Uses outputs generated during the spam, to create deep UTXO and branch structures.",
			},
		},
		{
			Name: "reuseLaterEnabled",
			Prompt: &survey.Select{
				Message: "Reuse outputs",
				Options: confirms,
				Default: defaultReuse,
				Help:    "Remember created outputs (add them to reuse outputs and use in future deep spams).",
			},
		},
	}
}

type spamDetailsSurvey struct {
	SpamDuration string
	SpamRate     string
	TimeUnit     string
}

var spamDetailsQuestions = func(defaultDuration, defaultRate, defaultTimeUnit string) []*survey.Question {
	return []*survey.Question{
		{
			Name: "spamDuration",
			Prompt: &survey.Input{
				Message: "Spam duration in [s].",
				Default: defaultDuration,
			},
		},
		{
			Name: "timeUnit",
			Prompt: &survey.Select{
				Message: "Choose time unit for the spam",
				Options: timeUnits,
				Default: defaultTimeUnit,
			},
		},
		{
			Name: "spamRate",
			Prompt: &survey.Input{
				Message: "Spam rate",
				Default: defaultRate,
			},
			Validate: func(val interface{}) error {
				if str, ok := val.(string); ok {
					_, err := strconv.Atoi(str)
					if err == nil {

						return nil
					}
					return errors.New("Incorrect spam rate")
				}
				return nil
			},
		},
	}
}

var spamScenarioQuestion = func(defaultScenario string) *survey.Select {
	return &survey.Select{
		Message: "Choose a spam scenario",
		Options: scenarios,
		Default: defaultScenario,
	}
}

var spamMenuQuestion = &survey.Select{
	Message: "Spam settings",
	Options: spamMenuOptions,
	Default: startSpam,
}

var currentMenuQuestion = &survey.Select{
	Options: currentSpamOptions,
	Default: back,
}

var removeSpammer = &survey.Input{
	Message: "Type in id of the spammer you wish to stop.",
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
