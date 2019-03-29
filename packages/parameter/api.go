package parameter

var (
    // expose int methods
    AddInt = addInt
    GetInt = getInt
    GetInts = getInts

    // expose string methods
    AddString = addString
    GetString = getString
    GetStrings = getStrings

    // expose events
    Events = moduleEvents{
        AddInt:    &intParameterEvent{make(map[uintptr]IntParameterConsumer)},
        AddString: &stringParameterEvent{make(map[uintptr]StringParameterConsumer)},
    }
)
