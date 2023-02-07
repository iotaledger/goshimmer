package generate

import (
	"os"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
)

// VariadicGenericsTemplate is a template that can be used to generate variadic generic implementations.
//
// It takes a file as input and creates multiple variadic instances of the source code replacing the following tokens:
//
//   - paramCount: the number of parameters of the generic variadic instance - i.e. 3
//   - constraints: list of type parameters without their constraints - i.e. "[T1, T2, T3]"
//   - Constraints: list of type parameters with their constraints - i.e. "[T1, T2, T3 any]"
//   - params: list of arguments without their type: "arg1, arg2, arg3"
//   - Params: list of arguments with their type: "arg1 T1, arg2 T2, arg3 T3"
//   - Types: list of types: "T1, T2, T3"
//
// Tokens have to be surrounded by /* and */ as comments in the source file. Neighboring whitespaces can be removed by
// adding a "-" in the beginning or the end of the token - see examples:
//
//   - "func NewEvent /*paramCount*/ () {" => "func NewEvent 3 () {"
//   - "func NewEvent /*-paramCount*/ () {" => "func NewEvent3 () {"
//   - "func NewEvent /*-paramCount-*/ () {" => "func NewEvent3() {"
type VariadicGenericsTemplate struct {
	// fixedHeader is the header of the file that is not supposed to be changed.
	fixedHeader string

	// dynamicContent is the content of the file that is supposed to be changed.
	dynamicContent string

	// tokenMappings is a map of tokens that are replaced with the given function.
	tokenMappings map[string]func(int) string
}

// NewVariadicGenericsTemplate creates a new VariadicGenericsTemplate from the given file.
func NewVariadicGenericsTemplate(fileName string) (*VariadicGenericsTemplate, error) {
	readFile, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	fixedHeader, dynamicContent, err := splitTemplate(string(readFile))
	if err != nil {
		return nil, err
	}

	return &VariadicGenericsTemplate{
		fixedHeader:    fixedHeader,
		dynamicContent: dynamicContent,
		tokenMappings: map[string]func(int) string{
			"paramCount":  func(i int) string { return "%d" },
			"constraints": func(i int) string { return "[" + buildVariadicString("T%d", i) + "]" },
			"Constraints": func(i int) string { return "[" + buildVariadicString("T%d", i) + " any]" },
			"params":      func(i int) string { return buildVariadicString("arg%d", i) },
			"Params":      func(i int) string { return buildVariadicString("arg%d T%d", i) },
			"Types":       func(i int) string { return buildVariadicString("T%d", i) },
		},
	}, nil
}

// Generate generates the file with the given name and the given number of variadic parameters.
func (v *VariadicGenericsTemplate) Generate(fileName string, paramCount int) error {
	targetFile, err := os.Create(fileName)
	if err != nil {
		return errors.Errorf("could not create file %s: %w", fileName, err)
	}
	defer func() { _ = targetFile.Close() }()

	if err = v.writeHeader(targetFile); err != nil {
		return errors.Errorf("could not write header to file %s: %w", fileName, err)
	}
	if err = v.writeContent(targetFile, paramCount); err != nil {
		return errors.Errorf("could not write content to file %s: %w", fileName, err)
	}

	return nil
}

// writeHeader writes the fixed header to the given file.
func (v *VariadicGenericsTemplate) writeHeader(targetFile *os.File) error {
	return lo.Return2(targetFile.WriteString("// Code generated automatically DO NOT EDIT.\n" + v.fixedHeader + "\n"))
}

// writeContent writes the dynamic content to the given file.
func (v *VariadicGenericsTemplate) writeContent(targetFile *os.File, paramCount int) error {
	for i := 1; i <= paramCount; i++ {
		_, err := targetFile.WriteString("\n" + v.replaceTokens(i) + "\n")
		if err != nil {
			return errors.Errorf("could not write content to file: %w", err)
		}
	}

	return nil
}

// replaceTokens translates the tokens in the dynamic content to the mapped ones from the tokenMappings.
func (v *VariadicGenericsTemplate) replaceTokens(argCount int) string {
	content := v.dynamicContent
	for token, replacement := range v.tokenMappings {
		content = strings.ReplaceAll(content, " /*-"+token+"-*/ ", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*-"+token+"-*/ ", "/*"+token+"*/")
		content = strings.ReplaceAll(content, " /*-"+token+"-*/", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*-"+token+"-*/", "/*"+token+"*/")
		content = strings.ReplaceAll(content, " /*-"+token+"*/", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*-"+token+"*/", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*"+token+"-*/ ", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*"+token+"-*/", "/*"+token+"*/")
		content = strings.ReplaceAll(content, "/*"+token+"*/", replacement(argCount))
	}

	return strings.ReplaceAll(content, "%d", strconv.Itoa(argCount))
}
