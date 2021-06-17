package config

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/mr-tron/base58"
)

func (config GoShimmer) CreateFlags() []string {
	var (
		enabledPlugins  = map[string]struct{}{}
		disabledPlugins = map[string]struct{}{}
		cmd             []string
	)

	for _, name := range config.DisabledPlugins {
		name = strings.ToLower(name)
		disabledPlugins[name] = struct{}{}
	}

	configVal := reflect.ValueOf(config)
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
		cmd = append(cmd, pluginCommands("--"+lowerCamelCase(field.Name)+".", configVal.Field(i))...)
	}

	cmd = append(
		[]string{
			"--node.enablePlugins=Webapi tools Endpoint", // TODO: Why doesn't this plugin have a proper name?
			fmt.Sprintf("--autopeering.seed=base58:%s", base58.Encode(config.Seed)),
			fmt.Sprintf("--node.enablePlugins=%s", setToString(enabledPlugins)),
			fmt.Sprintf("--node.disablePlugins=%s", setToString(disabledPlugins)),
		},
		cmd...)
	return cmd
}

func pluginCommands(prefix string, val reflect.Value) []string {
	var s []string
	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		if field.Name == "Enabled" {
			continue
		}

		switch field.Type.Kind() {
		case reflect.Struct:
			s = append(s, pluginCommands(prefix+lowerCamelCase(field.Name)+".", val.Field(i))...)
		default:
			s = append(s, fmt.Sprintf("%s%s=%s", prefix, lowerCamelCase(field.Name), toString(val.Field(i))))
		}
	}
	return s
}

func toString(v reflect.Value) string {
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
