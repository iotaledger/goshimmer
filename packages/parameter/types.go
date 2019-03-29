package parameter

type IntParameter struct {
    Name         string
    Value        *int
    DefaultValue int
    Description  string
}

type StringParameter struct {
    Name         string
    Value        *string
    DefaultValue string
    Description  string
}

type IntParameterConsumer = func(param *IntParameter)

type StringParameterConsumer = func(param *StringParameter)
