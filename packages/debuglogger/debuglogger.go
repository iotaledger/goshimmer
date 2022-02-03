package debuglogger

import (
	"fmt"
	"strings"
)

type DebugLogger struct {
	identifier          string
	indentation         int
	methodStartedBefore bool
}

func New(identifier string) (newDebugLogger *DebugLogger) {
	return &DebugLogger{
		identifier: identifier,
	}
}

func (d *DebugLogger) Println(params ...interface{}) {
	if d.methodStartedBefore {
		fmt.Print("[" + d.identifier + "] " + strings.Repeat("    ", d.Indentation()-1) + "{\n")
		d.methodStartedBefore = false
	}

	var stringParams string
	for _, param := range params {
		stringParams += fmt.Sprintf("%v", param)
	}

	result := ""
	for i, s := range strings.Split(stringParams, "\n") {
		if i == 0 {
			result += "[" + d.identifier + "] " + strings.Repeat("    ", d.Indentation()) + s + "\n"
		} else {
			result += "[" + d.identifier + "] " + strings.Repeat("    ", d.Indentation()+1) + s + "\n"
		}
	}

	fmt.Print(result)
}

func (d *DebugLogger) MethodStart(objectName, methodName string, params ...interface{}) (self *DebugLogger) {
	if d.methodStartedBefore {
		fmt.Print("[" + d.identifier + "] " + strings.Repeat("    ", d.Indentation()-1) + "{\n")
		d.methodStartedBefore = false
	}

	x := make([]interface{}, 0)
	x = append(x, objectName, ".", methodName, "(")
	for i, param := range params {
		if i != 0 {
			x = append(x, ", ")
		}

		x = append(x, d.paramString(param))
	}
	x = append(x, ")")

	d.Println(x...)
	d.IncreaseIndentation()
	d.methodStartedBefore = true

	return d
}

func (d *DebugLogger) MethodEnd() {
	d.DecreaseIndentation()

	if d.methodStartedBefore {
		d.methodStartedBefore = false
		return
	}

	d.Println("}")
}

func (d *DebugLogger) Indentation() (indentation int) {
	return d.indentation
}

func (d *DebugLogger) IncreaseIndentation() (self *DebugLogger) {
	d.indentation = d.indentation + 1

	return d
}

func (d *DebugLogger) DecreaseIndentation() (self *DebugLogger) {
	d.indentation = d.indentation - 1

	return d
}

func (d *DebugLogger) paramString(param interface{}) (paramString string) {
	switch typedParam := param.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", param)
	case fmt.Stringer:
		return typedParam.String()
	default:
		return fmt.Sprintf("%T(%v)", param, param)
	}
}
