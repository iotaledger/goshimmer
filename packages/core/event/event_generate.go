//go:build ignore

package main

import (
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		panic("expected one argument")
	}

	targetParamCount, err := strconv.Atoi(os.Args[1])
	if err != nil {
		panic(err)
	}

	targetFile, err := os.Create("event.go")
	if err != nil {
		panic(err)
	}

	readFile, err := os.ReadFile(os.Getenv("GOFILE"))
	if err != nil {
		panic(err)
	}

	fileContent := string(readFile)

	splitTemplate := strings.Split(fileContent, "//go:generate")
	if len(splitTemplate) != 2 {
		panic("could not find go:generate directive")
	}

	header := strings.TrimSpace(strings.ReplaceAll(splitTemplate[0], "//go:build ignore", ""))
	footer := splitTemplate[1]
	footer = strings.TrimSpace(footer[strings.Index(footer, "\n"):])

	targetFile.WriteString("// Code generated automatically DO NOT EDIT.\n" + header + "\n")

	for paramCount := 0; paramCount <= targetParamCount; paramCount++ {
		if paramCount == 0 {
			targetFile.WriteString("\n" + replaceTokens(footer, map[string]string{
				"paramCount":  "",
				"ParamCount":  "no",
				"constraints": "",
				"Constraints": "",
				"params":      "",
				"Params":      "",
				"Types":       "",
			}) + "\n")
		} else {
			targetFile.WriteString("\n" + replaceTokens(footer, map[string]string{
				"paramCount":  strconv.Itoa(paramCount),
				"ParamCount":  strconv.Itoa(paramCount),
				"constraints": "[" + buildArgStrings("T%d", paramCount) + "]",
				"Constraints": "[" + buildArgStrings("T%d", paramCount) + " any]",
				"params":      buildArgStrings("arg%d", paramCount),
				"Params":      buildArgStrings("arg%d T%d", paramCount),
				"Types":       buildArgStrings("T%d", paramCount),
			}) + "\n")
		}
	}

	if err = targetFile.Close(); err != nil {
		panic(err)
	}
}

func replaceTokens(content string, replacements map[string]string) string {
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

	return content
}

func buildArgStrings(template string, end int) string {
	results := make([]string, 0)
	for i := 1; i <= end; i++ {
		results = append(results, strings.ReplaceAll(template, "%d", strconv.Itoa(i)))
	}

	return strings.Join(results, ", ")
}
