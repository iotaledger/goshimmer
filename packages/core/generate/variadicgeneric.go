package generate

import (
	"os"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

func VariadicGeneric(sourceFileName string, targetFileName string, targetParamCount int) error {
	targetFile, err := os.Create(targetFileName)
	if err != nil {
		return err
	}

	readFile, err := os.ReadFile(sourceFileName)
	if err != nil {
		return err
	}

	splitTemplate := strings.Split(string(readFile), "//go:generate")
	if len(splitTemplate) != 2 {
		return errors.New("could not find go:generate directive")
	}

	header := strings.TrimSpace(strings.ReplaceAll(splitTemplate[0], "//go:build ignore", ""))
	footer := splitTemplate[1]
	footer = strings.TrimSpace(footer[strings.Index(footer, "\n"):])

	_, err = targetFile.WriteString("// Code generated automatically DO NOT EDIT.\n" + header + "\n")
	if err != nil {
		return err
	}

	for paramCount := 0; paramCount <= targetParamCount; paramCount++ {
		if paramCount == 0 {
			_, err := targetFile.WriteString("\n" + replaceTokens(footer, map[string]string{
				"paramCount":  "",
				"ParamCount":  "no",
				"constraints": "",
				"Constraints": "",
				"params":      "",
				"Params":      "",
				"Types":       "",
			}, paramCount) + "\n")
			if err != nil {
				return err
			}
		} else {
			_, err := targetFile.WriteString("\n" + replaceTokens(footer, map[string]string{
				"paramCount":  "%d",
				"ParamCount":  "%d",
				"constraints": "[" + buildArgStrings("T%d", paramCount) + "]",
				"Constraints": "[" + buildArgStrings("T%d", paramCount) + " any]",
				"params":      buildArgStrings("arg%d", paramCount),
				"Params":      buildArgStrings("arg%d T%d", paramCount),
				"Types":       buildArgStrings("T%d", paramCount),
			}, paramCount) + "\n")
			if err != nil {
				return err
			}
		}
	}

	if err = targetFile.Close(); err != nil {
		return err
	}

	return nil
}

func replaceTokens(content string, replacements map[string]string, argCount int) string {
	for token, replacement := range replacements {
		content = strings.ReplaceAll(content, " /*-"+token+"-*/ ", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*-"+token+"-*/ ", "/*"+token+"*/")
		content = strings.ReplaceAll(content, " /*-"+token+"-*/", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*-"+token+"-*/", "/*"+token+"*/")
		content = strings.ReplaceAll(content, " /*-"+token+"*/", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*-"+token+"*/", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*"+token+"-*/ ", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*"+token+"-*/", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*"+token+"*/", replacement)
	}

	return strings.ReplaceAll(content, "%d", strconv.Itoa(argCount))
}

func buildArgStrings(template string, end int) string {
	results := make([]string, 0)
	for i := 1; i <= end; i++ {
		results = append(results, strings.ReplaceAll(template, "%d", strconv.Itoa(i)))
	}

	return strings.Join(results, ", ")
}
