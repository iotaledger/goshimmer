package main

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/cockroachdb/errors"
	"strconv"
)

// region survey //////////////////////////////////////////////////////////////////////////////////////////////

var actionQuestion = &survey.Select{
	Message: "Choose an action",
	Options: actions,
	Default: "Evil wallet details",
}

var fundsQuestion = &survey.Select{
	Message: "How many fresh outputs you want to create?",
	Options: []string{"100", "10000", "50000", "100000", "cancel"},
	Default: "100",
}

type settingSurvey struct {
	FundsCreation string
}

var settingsQuestion = []*survey.Question{
	{
		Name: "fundsCreation",
		Prompt: &survey.Select{
			Message: "Enable automatic faucet output creation",
			Options: []string{"enable", "disable"},
			Default: "enable",
		},
	},
}

type spamSettingSurvey struct {
	SpamType          string
	DeepSpamEnabled   bool
	ReuseLaterEnabled bool
}

var spamQuestions = []*survey.Question{
	{
		Name: "deepSpamEnabled",
		Prompt: &survey.Select{
			Message: "Enable deep spam?",
			Options: []string{"enable", "disable"},
			Default: "disable",
		},
	},
	{
		Name: "reuseLaterEnabled",
		Prompt: &survey.Select{
			Message: "Remember created outputs (add them to reuse outputs and use in future deep spams).",
			Options: []string{"enable", "disable"},
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

var scenarioQuestion = &survey.Select{
	Message: "Choose a spam scenario",
	Options: scenarios,
	Default: "guava",
}

// endregion //////////////////////////////////////////////////////////////////////////////////////////////
