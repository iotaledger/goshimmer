package config

import (
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

// fillStructFromDefaultTag recursively explores the given struct pointer and sets values of fields to the `default` as
// specified in the struct tags.
func fillStructFromDefaultTag(val reflect.Value) {
	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)

		if valueField.Kind() == reflect.Struct {
			fillStructFromDefaultTag(valueField)
			continue
		}

		tagDefaultValue, exists := typeField.Tag.Lookup("default")
		if !exists {
			continue
		}

		switch valueField.Interface().(type) {
		case string: // no conversion needed
			valueField.SetString(tagDefaultValue)
		case []string: // parse comma separated strings instead of JSON-style lists
			defaultValue, err := csv.NewReader(strings.NewReader(tagDefaultValue)).Read()
			if err != nil {
				panic(err)
			}
			valueField.Set(reflect.ValueOf(defaultValue))
		case time.Duration: // Duration does not implement encoding.TextUnmarshaler, so we have to do it manually
			defaultValue, err := time.ParseDuration(tagDefaultValue)
			if err != nil {
				panic(err)
			}
			valueField.Set(reflect.ValueOf(defaultValue))
		default: // use the JSON unmarshaler for everything else
			if err := json.Unmarshal([]byte(tagDefaultValue), valueField.Addr().Interface()); err != nil {
				panic(err)
			}
		}
	}
}
