# golangci-lint

## Overview

We use golangci-lint v1.38.0 to run various types of linters on our codebase. All settings are stored in the `.golangci.yml` file.

golangci-lint is very flexible and customizable. Check the [official golangci-lint docs](https://golangci-lint.run/usage/configuration/) to see how configuration works 

## How To Run

1. Install the golangci-lint program following the [official installation i instructions](https://golangci-lint.run/usage/install/).
2. In the project root, run : 
   
```bash
golangci-lint run
```

## Dealing With Errors

Most of the errors that golangci-lint reports are errors from formatting linters like `gofmt`, and `goimports`. You can easily auto-fix them with:

```bash
golangci-lint run --fix
```

Here is the full list of linters that support the auto-fix feature: 

* [gofmt](https://pkg.go.dev/cmd/gofmt)
* [gofumpt](https://pkg.go.dev/github.com/vearutop/gofumpt)
* [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports)
* [misspell](https://pkg.go.dev/github.com/766b/gometalinter/_linters/src/github.com/client9/misspell/cmd/misspell)
* [whitespace](https://github.com/ultraware/whitespace)

In case it is not a formatting error, do your best to fix it first. If you think it is a false alarm, there are a few ways how to disable that check in golangci-lint:

- Exclude the check by the error text regexp. Example: `'Error return value of .((os\.)?std(out|err)\..*|.*Close|.*Flush|os\.Remove(All)?|.*print(f|ln)?|os\.(Un)?Setenv). is not checked'`.
- Exclude the entire linter for that file type. Example: don't run `errcheck` in Go test files.
- Change linter settings to make it more relaxed. 
- Disable that particular error occurrence: use a comment with a special `nolint` directive next to the place in code with the error. Example: `// nolint: errcheck`.

# Docker

## Building an Image

We use the new buildkit docker engine to build a `iotaledger/goshimmer` image.  The minimum required docker version that supports this feature is `18.09`. 

To enable buildkit engine in your local docker, add the following to the docker configuration json file:

```json
{ "features": { "buildkit": true } }
```

You can find detailed instructions on how to build the image using buildkit in this [article](https://docs.docker.com/develop/develop-images/build_enhancements/#to-enable-buildkit-builds).

### Troubleshooting


```dockerfile
Step 10/17 : RUN --mount=target=.     --mount=type=cache,target=/root/.cache/go-build     CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build     -ldflags='-w -s -extldflags "-static"'     -o /go/bin/goshimmer;     ./check_static.sh
 ---> Running in ecdae1c9339d
no Go files in /goshimmer
/bin/sh: 1: ./check_static.sh: not found
The command '/bin/sh -c CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build     -ldflags='-w -s -extldflags "-static"'     -o /go/bin/goshimmer;     ./check_static.sh' returned a non-zero code: 127
```

If you are getting a similar error during the docker build, it means that buildkit feature doesn't work in your docker.

If you already enabled it in the configuration json file as described above, and your docker version is `18.09` or higher, try to set the following env variables when building the docker image:

```
DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker build -t iotaledger/goshimmer .
```
