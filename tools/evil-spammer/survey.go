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
	Help:    "Provide valid API url",
}

var removeUrlQuestion = func(urls []string) *survey.MultiSelect {
	return &survey.MultiSelect{
		Message: "Which ",
		Options: urls,
	}
}

type spamTypeSurvey struct {
	DeepSpamEnabled   string
	ReuseLaterEnabled string
}

var spamTypeQuestions = []*survey.Question{
	{
		Name: "deepSpamEnabled",
		Prompt: &survey.Select{
			Message: "Enable deep spam?",
			Options: confirms,
			Default: "disable",
		},
	},
	{
		Name: "reuseLaterEnabled",
		Prompt: &survey.Select{
			Message: "Remember created outputs (add them to reuse outputs and use in future deep spams).",
			Options: confirms,
			Default: "enable",
		},
	},
}

type spamDetailsSurvey struct {
	SpamDuration string
	SpamRate     string
}

var spamDetailsQuestions = []*survey.Question{
	{
		Name: "spamDuration",
		Prompt: &survey.Input{
			Message: "Duration of the spam in seconds. Max spam duration: 600.",
			Default: "60",
		},
		Validate: func(val interface{}) error {
			if str, ok := val.(string); ok {
				n, err := strconv.Atoi(str)
				if err == nil {
					if n <= 600 {
						return nil
					}
				}
				return errors.New("Incorrect spam duration. Provide duration in seconds.")
			}
			return nil
		},
	},
	{
		Name: "spamRate",
		Prompt: &survey.Input{
			Message: "Provide the rate of the spam in message/tx/batch per second.",
			Default: "5",
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

var spamScenarioQuestion = &survey.Select{
	Message: "Choose a spam scenario",
	Options: scenarios,
	Default: "guava",
}

var spamMenuQuestion = &survey.Select{
	Message: "Spam settings",
	Options: spamMenuOptions,
	Default: spamDetails,
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
