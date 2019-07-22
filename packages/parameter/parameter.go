package parameter

var boolParameters = make(map[string]*BoolParameter)

func AddBool(name string, defaultValue bool, description string) *BoolParameter {
	if boolParameters[name] != nil {
		panic("duplicate parameter - \"" + name + "\" was defined already")
	}

	newParameter := &BoolParameter{
		Name:         name,
		DefaultValue: defaultValue,
		Value:        &defaultValue,
		Description:  description,
	}

	boolParameters[name] = newParameter

	Events.AddBool.Trigger(newParameter)

	return newParameter
}

func GetBool(name string) *BoolParameter {
	return boolParameters[name]
}

func GetBools() map[string]*BoolParameter {
	return boolParameters
}

var intParameters = make(map[string]*IntParameter)

func AddInt(name string, defaultValue int, description string) *IntParameter {
	if intParameters[name] != nil {
		panic("duplicate parameter - \"" + name + "\" was defined already")
	}

	newParameter := &IntParameter{
		Name:         name,
		DefaultValue: defaultValue,
		Value:        &defaultValue,
		Description:  description,
	}

	intParameters[name] = newParameter

	Events.AddInt.Trigger(newParameter)

	return newParameter
}

func GetInt(name string) *IntParameter {
	return intParameters[name]
}

func GetInts() map[string]*IntParameter {
	return intParameters
}

var stringParameters = make(map[string]*StringParameter)

func AddString(name string, defaultValue string, description string) *StringParameter {
	if stringParameters[name] != nil {
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

func GetString(name string) *StringParameter {
	return stringParameters[name]
}

func GetStrings() map[string]*StringParameter {
	return stringParameters
}
