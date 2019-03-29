package parameter

var intParameters = make(map[string]*IntParameter)

func addInt(name string, defaultValue int, description string) *IntParameter {
    if intParameters[name] != nil {
        panic("duplicate parameter - \"" + name + "\" was defined already")
    }

    newParameter := &IntParameter{
        Name:          name,
        DefaultValue:  defaultValue,
        Value:         &defaultValue,
        Description:   description,
    }

    intParameters[name] = newParameter

    Events.AddInt.Trigger(newParameter)

    return newParameter
}

func getInt(name string) *IntParameter {
    return intParameters[name]
}

func getInts() map[string]*IntParameter {
    return intParameters
}

var stringParameters  = make(map[string]*StringParameter)

func addString(name string, defaultValue string, description string) *StringParameter {
    if intParameters[name] != nil {
        panic("duplicate parameter - \"" + name + "\" was defined already")
    }

    newParameter := &StringParameter{
        Name:         name,
        DefaultValue: defaultValue,
        Value:        &defaultValue,
        Description:  description,
    }

    stringParameters[name] = newParameter

    Events.AddString.Trigger(newParameter)

    return stringParameters[name]
}

func getString(name string) *StringParameter {
    return stringParameters[name]
}

func getStrings() map[string]*StringParameter {
    return stringParameters
}
