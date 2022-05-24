---
description: How to run and use golangci-lint to lint your code. How to build an image with the buildkit docker engine. 
image: /img/logo/goshimmer_light.png
keywords:
- port config
- golang
- lint
- error handling
- golangci-lint
- docker
- buildkit
- image
- configuration json
---
# golangci-lint

## Overview

We use golangci-lint v1.38.0 to run various types of linters on our codebase. All settings are stored in the `.golangci.yml` file.
golangci-lint is very flexible and customizable. Check the docs to see how configuration works https://golangci-lint.run/usage/configuration/

## How to Run

1. Install the golangci-lint program https://golangci-lint.run/usage/install/
2. In the project root: `golangci-lint run`

## Dealing With Errors
Most of the errors that golangci-lint reports are errors from formatting linters like `gofmt`, `goimports` and etc. You can easily auto-fix them with:
```shell
golangci-lint run --fix
```

Here is the full list of linters that support the auto-fix feature: `gofmt`, `gofumpt`, `goimports`, `misspell`, `whitespace`.

In case it's not a formatting error, do your best to fix it first. If you think it's a false alarm there are a few ways how to disable that check in golangci-lint:
- Exclude the check by the error text regexp. Example: `'Error return value of .((os\.)?std(out|err)\..*|.*Close|.*Flush|os\.Remove(All)?|.*print(f|ln)?|os\.(Un)?Setenv). is not checked'`.
- Exclude the entire linter for that file type. Example: don't run `errcheck` in Go test files.
- Change linter settings to make it more relaxed. 
- Disable that particular error occurrence: use a comment with a special `nolint` directive next to the place in code with the error. Example: `// nolint: errcheck`.

# Docker

## Building an Image

We use the new buildkit docker engine to build `iotaledger/goshimmer` image. 
The minimum required docker version that supports this feature is `18.09`. 
To enable buildkit engine in your local docker add the following to the docker configuration json file:
```json
{ "features": { "buildkit": true } }
```
Check this [article](https://docs.docker.com/develop/develop-images/build_enhancements/#to-enable-buildkit-builds) for details on how to do that.

### Troubleshooting

If you already enabled the buildkit engine in the configuration json file as described above and docker version is `18.09` or higher,
try to set the following env variables when building the docker image:

```shell
DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker build -t iotaledger/goshimmer .
```



