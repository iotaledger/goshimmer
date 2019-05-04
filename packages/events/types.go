package events

func CallbackCaller(handler interface{}, params ...interface{}) { handler.(func())() }

func ErrorCaller(handler interface{}, params ...interface{}) { handler.(func(error))(params[0].(error)) }
