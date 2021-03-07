package configuration

import (
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/pflag"
)

func DefineParameters(parameters interface{}, optionalPrefix ...string) {
	var parameterPrefix string
	if len(optionalPrefix) == 0 {
		parameterPrefix = lowerCamelCase(callerShortPackageName())
	} else {
		parameterPrefix = optionalPrefix[0]
	}

	val := reflect.ValueOf(parameters).Elem()
	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)

		valueAddr := valueField.Addr().Interface()
		var name string
		if customName := typeField.Tag.Get("name"); customName != "" {
			name = customName
		} else {
			name = lowerCamelCase(typeField.Name)
		}
		usage := typeField.Tag.Get("usage")

		switch valueField.Interface().(type) {
		case bool:
			defaultValue, err := strconv.ParseBool(typeField.Tag.Get("default"))
			if err != nil {
				panic(err)
			}
			pflag.BoolVar(valueAddr.(*bool), parameterPrefix+"."+name, defaultValue, usage)
		case int:
			defaultValue, err := strconv.Atoi(typeField.Tag.Get("default"))
			if err != nil {
				panic(err)
			}
			pflag.IntVar(valueAddr.(*int), parameterPrefix+"."+name, defaultValue, usage)
		case string:
			defaultValue := typeField.Tag.Get("default")
			pflag.StringVar(valueAddr.(*string), parameterPrefix+"."+name, defaultValue, usage)
		case int64:
			defaultValue, err := strconv.ParseInt(typeField.Tag.Get("default"), 10, 64)
			if err != nil {
				panic(err)
			}
			pflag.Int64Var(valueAddr.(*int64), parameterPrefix+"."+name, defaultValue, usage)
		default:
			DefineParameters(valueAddr, parameterPrefix+"."+name)
		}
	}
}

func lowerCamelCase(str string) string {
	runes := []rune(str)
	runeCount := len(runes)

	if runeCount == 0 || unicode.IsLower(runes[0]) {
		return str
	}

	runes[0] = unicode.ToLower(runes[0])
	if runeCount == 1 || unicode.IsLower(runes[1]) {
		return string(runes)
	}

	for i := 1; i < runeCount; i++ {
		if i+1 < runeCount && unicode.IsLower(runes[i+1]) {
			break
		}

		runes[i] = unicode.ToLower(runes[i])
	}

	return string(runes)
}

func callerShortPackageName() string {
	pc, _, _, _ := runtime.Caller(2)
	funcName := runtime.FuncForPC(pc).Name()
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash < 0 {
		lastSlash = 0
	}
	firstDot := strings.IndexByte(funcName[lastSlash:], '.') + lastSlash

	return funcName[lastSlash+1 : firstDot]
}
