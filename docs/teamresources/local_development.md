# golangci-lint

## Overview

We use golangci-lint v1.38.0 to run various types of linters on our codebase. All settings are stored in the `.golangci.yml` file.
golangci-lint is very flexible and customizable. Check the docs to see how configuration works https://golangci-lint.run/usage/configuration/

## How to run

1. Install the golangci-lint program https://golangci-lint.run/usage/install/
2. In the project root: `golangci-lint run`

## Dealing with errors
Most of the errors that golangci-lint reports are errors from formatting linters like `gofmt`, `goimports` and etc. You can easily auto-fix them with:
```
golangci-lint run --fix
```

Here is the full list of linters that support the auto-fix feature: `gofmt`, `gofumpt`, `goimports`, `misspell`, `whitespace`.

In case it's not a formatting error, do your best to fix it first. If you think it's a false alarm there are a few ways how to disable that check in golangci-lint:
- Exclude the check by the error text regexp. Example: `'Error return value of .((os\.)?std(out|err)\..*|.*Close|.*Flush|os\.Remove(All)?|.*print(f|ln)?|os\.(Un)?Setenv). is not checked'`.
- Exclude the entire linter for that file type. Example: don't run `errcheck` in Go test files.
- Change linter settings to make it more relaxed. 
- Disable that particular error occurrence: use a comment with a special `nolint` directive next to the place in code with the error. Example: `// nolint: errcheck`.
