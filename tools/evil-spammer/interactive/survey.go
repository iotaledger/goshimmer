package interactive

import "github.com/AlecAivazis/survey/v2"

// region survey //////////////////////////////////////////////////////////////////////////////////////////////

type actionSurvey struct {
	Action string
}

// the questions to ask
var actionQuestion = []*survey.Question{
	{
		Name: "action",
		Prompt: &survey.Select{
			Message: "Choose an action",
			Options: actions,
			Default: "spam",
		},
	},
}

// endregion //////////////////////////////////////////////////////////////////////////////////////////////
