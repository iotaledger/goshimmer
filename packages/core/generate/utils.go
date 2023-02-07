package generate

import (
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

// splitTemplate splits the given template into a fixed header and a dynamic content part that can be generated.
func splitTemplate(template string) (fixedHeader, dynamicContent string, err error) {
	if splitTemplate := strings.Split(template, "//go:generate"); len(splitTemplate) == 2 {
		fixedHeader = strings.TrimSpace(strings.ReplaceAll(splitTemplate[0], "//go:build ignore", ""))
		dynamicContent = strings.TrimSpace(splitTemplate[1][strings.Index(splitTemplate[1], "\n"):])

		return fixedHeader, dynamicContent, nil
	}

	return "", "", errors.New("could not find go:generate directive")
}

// buildVariadicString builds a comma separated concatenation of the template, with each instance having the token %d
// being replaced with the  number of the iteration.
//
// Example: buildVariadicString("func(%d)", 3) => "func(1), func(2), func(3)"
func buildVariadicString(template string, upperBound int) string {
	results := make([]string, 0)
	for i := 1; i <= upperBound; i++ {
		results = append(results, strings.ReplaceAll(template, "%d", strconv.Itoa(i)))
	}

	return strings.Join(results, ", ")
}
