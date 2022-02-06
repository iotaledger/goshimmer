package debuglogger

import (
	"fmt"
	"strings"
)

// DebugLogger is a logger that supports log outputs with indentation to reflect the structure and hierarchical nature
// of executed functions and code blocks.
type DebugLogger struct {
	identifier          string
	indentation         int
	methodStartedBefore bool
}

// New creates a new DebugLogger instance with the given identifier.
func New(identifier string) (newDebugLogger *DebugLogger) {
	return &DebugLogger{
		identifier: identifier,
	}
}

// Println logs a message to the console.
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

// MethodStart logs a method call and indents the following log outputs. The indentation is reverted after a call of
// MethodEnd.
//
// Usage: defer logger.MethodStart("myObject", "myMethod", param1, param2).MethodEnd() // at beginning of the method.
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

// MethodEnd logs the end of the method (closing curly braces) and decreases the indentation by 1 level.
func (d *DebugLogger) MethodEnd() {
	d.DecreaseIndentation()

	if d.methodStartedBefore {
		d.methodStartedBefore = false
		return
	}

	d.Println("}")
}

// Indentation returns the current indentation of the DebugLogger.
func (d *DebugLogger) Indentation() (indentation int) {
	return d.indentation
}

// IncreaseIndentation increases the indentation of the DebugLogger.
func (d *DebugLogger) IncreaseIndentation() (self *DebugLogger) {
	d.indentation++

	return d
}

// DecreaseIndentation decreases the indentation of the DebugLogger.
func (d *DebugLogger) DecreaseIndentation() (self *DebugLogger) {
	d.indentation--

	return d
}

// paramString is an internal utility function that generates the output of a method parameter.
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
