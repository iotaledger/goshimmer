package config

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/mr-tron/base58"
)

// CreateFlags converts a GoShimmer config into the corresponding command line flags.
func (s GoShimmer) CreateFlags() []string {
	var (
		enabledPlugins  = map[string]struct{}{}
		disabledPlugins = map[string]struct{}{}
		flags           []string
	)

	for _, name := range s.DisabledPlugins {
		name = strings.ToLower(name)
		disabledPlugins[name] = struct{}{}
	}

	configVal := reflect.ValueOf(s)
	for i := 0; i < configVal.NumField(); i++ {
		field := configVal.Type().Field(i)
		if field.Type.Kind() != reflect.Struct {
			continue
		}

		enabled, ok := configVal.Field(i).FieldByName("Enabled").Interface().(bool)
		if !ok {
			panic("each plugin config struct must contain a bool `Enabled` field")
		}
		pluginName := strings.ToLower(field.Name)
		if !enabled {
			disabledPlugins[pluginName] = struct{}{}
			delete(enabledPlugins, pluginName)
			continue
		}
		enabledPlugins[pluginName] = struct{}{}
		delete(disabledPlugins, pluginName)
		flags = append(flags, pluginCommands("--"+lowerCamelCase(field.Name)+".", configVal.Field(i))...)
	}

	flags = append(
		[]string{
			"--node.overwriteStoredSeed=true",
			fmt.Sprintf("--node.enablePlugins=%s", setToString(enabledPlugins)),
			fmt.Sprintf("--node.disablePlugins=%s", setToString(disabledPlugins)),
		},
		flags...)

	// manually add seed to flags if autopeering is disabled
	// this is necessary due to the fact that the seed is currently still part of the autopeering configuration
	var seedProvided bool
	for _, f := range flags {
		if strings.Contains(f, "node.seed") {
			seedProvided = true
		}
	}
	if !seedProvided {
		flags = append(flags, fmt.Sprintf("--node.seed=base58:%s", base58.Encode(s.Seed)))
	}

	return flags
}

func pluginCommands(prefix string, val reflect.Value) []string {
	var s []string
	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		if field.Name == "Enabled" {
			continue
		}

		name := lowerCamelCase(field.Name)
		if value, ok := field.Tag.Lookup("name"); ok {
			name = value
		}

		switch field.Type.Kind() {
		case reflect.Struct:
			if strings.Contains(field.Name, "ParametersDefinition") {
				s = append(s, pluginCommands(prefix, val.Field(i))...)
			} else {
				s = append(s, pluginCommands(prefix+name+".", val.Field(i))...)
			}
		default:
			s = append(s, fmt.Sprintf("%s%s=%s", prefix, name, valueToString(val.Field(i))))
		}
	}
	return s
}

func valueToString(v reflect.Value) string {
	switch v.Type().Kind() {
	case reflect.Slice, reflect.Array:
		var b strings.Builder
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(fmt.Sprintf("%v", v.Index(i)))
		}
		return b.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func lowerCamelCase(s string) string {
	for i, r := range s {
		if unicode.IsLower(r) {
			if i <= 1 {
				return strings.ToLower(s[:i]) + s[i:]
			}
			return strings.ToLower(s[:i-1]) + s[i-1:]
		}
	}
	return strings.ToLower(s)
}

func setToString(set map[string]struct{}) string {
	slice := make([]string, 0, len(set))
	for key := range set {
		slice = append(slice, key)
	}
	return strings.Join(slice, ",")
}
