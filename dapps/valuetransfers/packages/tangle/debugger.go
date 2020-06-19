package tangle

import (
	"fmt"
	"strings"
)

// Debugger represents a utility that allows us to print debug messages and function calls.
type Debugger struct {
	aliases map[interface{}]string
	enabled bool
	indent  int
}

// NewDebugger is the constructor of a debugger instance.
func NewDebugger() *Debugger {
	return (&Debugger{}).ResetAliases()
}

// Enable sets the debugger to print the debug information.
func (debugger *Debugger) Enable() {
	debugger.enabled = true

	fmt.Println("[DEBUGGER::ENABLED]")
}

// Disable sets the debugger to not print any debug information.
func (debugger *Debugger) Disable() {
	fmt.Println("[DEBUGGER::DISABLED]")
	debugger.enabled = false
}

// ResetAliases removes any previously registered aliases. This can be useful if the same debugger instance is for
// example used in different tests or test cases.
func (debugger *Debugger) ResetAliases() *Debugger {
	debugger.aliases = make(map[interface{}]string)

	return debugger
}

// RegisterAlias registers a string representation for the given element. This can be used to create a string
// representation for things like ids in the form of byte slices.
func (debugger *Debugger) RegisterAlias(element interface{}, alias string) {
	debugger.aliases[element] = alias
}

// FunctionCall prints debug information about a function call. It automatically indents all following debug outputs
// until Return() is called. The best way to use this is by starting a function call with a construct like:
//
// defer debugger.FunctionCall("myFunction", param1, param2).Return()
func (debugger *Debugger) FunctionCall(identifier string, params ...interface{}) *Debugger {
	if !debugger.enabled {
		return debugger
	}

	debugger.Print(identifier + "(" + debugger.paramsAsCommaSeparatedList(params...) + ") {")
	debugger.indent++

	return debugger
}

// Return prints debug information about a FunctionCall() the was finished. It reduces the indentation for consecutive
// debug outputs.
func (debugger *Debugger) Return() *Debugger {
	if !debugger.enabled {
		return debugger
	}

	debugger.indent--
	debugger.Print("}")

	return debugger
}

// Print prints an arbitrary debug message that can for example be used to print an information when a certain part of
// the code is executed.
func (debugger *Debugger) Print(identifier string, params ...interface{}) {
	if !debugger.enabled {
		return
	}

	if len(params) >= 1 {
		debugger.print(identifier + " = " + debugger.paramsAsCommaSeparatedList(params...))
	} else {
		debugger.print(identifier)
	}
}

// print is an internal utility function that actually prints the given string to stdout.
func (debugger *Debugger) print(stringToPrint string) {
	fmt.Println("[DEBUGGER] " + strings.Repeat("    ", debugger.indent) + stringToPrint)
}

// paramsAsCommaSeparatedList creates a comma separated list of the given parameters.
func (debugger *Debugger) paramsAsCommaSeparatedList(params ...interface{}) string {
	paramsAsStrings := make([]string, len(params))
	for i, param := range params {
		paramsAsStrings[i] = debugger.paramAsString(param)
	}

	return strings.Join(paramsAsStrings, ", ")
}

// paramAsString returns a string representation of an arbitrary parameter.
func (debugger *Debugger) paramAsString(param interface{}) string {
	defer func() { recover() }()
	if alias, aliasExists := debugger.aliases[param]; aliasExists {
		return alias
	}

	return fmt.Sprint(param)
}

// debugger contains the default global debugger instance.
var debugger = NewDebugger()
