package events

func CallbackCaller(handler interface{}, params ...interface{}) { handler.(func())() }
