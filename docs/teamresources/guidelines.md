# Code guidelines
## General guidelines

- Don’t use `log.Fatal()` or `os.Exit()` outside of the main. It immediately terminates the program and all defers are ignored and no graceful shutdown is possible. It can lead to inconsistencies. Propagate the error up to the main and let the main function exit instead. Avoid panics as well, almost always use errors. [Example](https://github.com/iotaledger/goshimmer/blob/f75ce47eeaa3bf930b368754ac24b72f768a5964/plugins/autopeering/autopeering.go#L135).
- Don’t duplicate code, reuse it. In tests too. Example: [duplicate1](https://github.com/iotaledger/goshimmer/blob/f75ce47eeaa3bf930b368754ac24b72f768a5964/packages/ledgerstate/branch_dag.go#L969) and [duplicate2](https://github.com/iotaledger/goshimmer/blob/f75ce47eeaa3bf930b368754ac24b72f768a5964/packages/ledgerstate/branch_dag.go#L1053)
- Unhandled errors can cause bugs and make it harder to diagnose problems. Try to handle all errors: propagate them to the caller or log them. Even if the function call is used with a defer, and it’s inconvenient to handle the error it returns, still handle it. Wrap the function call in an anonymous function assign error to the upper error  like that:
```go
    defer func() {
        cerr := f.Close()
        if err == nil {
            err = xerrors.Errorf("failed to close file: %w", cerr)
        }
    }()
```
- Wrap errors with `xerrors.Errorf()` when returning them to the caller. It adds the stack trace and a custom message to the error. Without that information investigating an issue is very hard.
- Use `xerrors.Is()` instead of direct errors comparison. This function unwraps errors recursively. [Example](https://github.com/iotaledger/goshimmer/pull/1113/files#diff-05fdc081489a8d5a61224d812f9bbd7bc77edf9769ed00d95ea024d2a44a699aL62).
- Propagate `ctx` and use APIs that accept `ctx`, start exposing APIs that accept `ctx`. Context is a native way for timeouts/cancellation in Go. It allows writing more resilient and fault tolerant code. [Example](https://github.com/iotaledger/goshimmer/pull/1113/files#diff-f2820ed0d3d4d9ea05b78b1dd3978dbcf9401c8caaa8cc40cc1c0342a55379fcL35).
- Don’t shadow builtin functions like copy, len, new etc. [Example](https://github.com/iotaledger/goshimmer/pull/1113/files#diff-f07268750a44da26386469c1b1e93574a678c3d418fce9e1f186d5f1991a92eaL14).
- Don’t shadow imported packages. [Example](https://github.com/iotaledger/goshimmer/blob/f75ce47eeaa3bf930b368754ac24b72f768a5964/plugins/webapi/value/sendtransactionbyjson.go#L172).
- Don’t do `[:]` on a slice. It has no effect. [Example](https://github.com/iotaledger/goshimmer/pull/1113/files#diff-299a1ac5fa09739ea07b7c806ee2785d83eea110f8af143dbc853a25e4819116L133).
- Avoid naked returns if the function isn’t very small. It makes the code more readable.
- Define explicit constants for strings that are used 3 times or more. It makes the code more maintainable.
- Define explicit constants for all numbers. It makes the code more readable.
- Don’t write really long and complex functions. Split them into smaller ones.
- Treat comments as regular text/documentation. Start with a capital letter, set space after `//` and end them with a dot. It’s a good habit since Go package docs are generated automatically from the comments and displayed on the godoc site.

## Error handling

We use the new error wrapping API and behavior introduced with Go 1.13 but we use the "golang.org/x/xerrors" drop-in replacement which follows the Go 2 design draft and which enables us to have a stack trace for every "wrapping" of the error.

Errors should always be wrapped an annotated with an additional message at each step. The following example shows how errors are wrapped and turned into the corresponding sentinel errors.

```go
package example

import (
    "errors"
    "3rdPartyLibrary"

    "golang.org/x/xerrors"
)

// define error variables to make errors identifiable (sentinel errors)
var ErrSentinel = errors.New("identifiable error")

// turn anonymous 3rd party errors into identifiable ones
func SentinelErrFrom3rdParty() (result interface{}, err error)
    if result, err = 3rdPartyLibrary.DoSomething(); err != nil {
        err = xerrors.Errorf("failed to do something (%v): %w", err, ErrSentinel)
        return
    }

    return
}

// wrap recursive errors at each step
func WrappedErrFromInternalCall() error {
    return xerrors.Errorf("wrapped internal error: %w", SentinelErrFrom3rdParty())
}

// create "new" identifiable internal errors that are not originating in 3rd party libs
func ErrFromInternalCall() error {
    return xerrors.Errorf("internal error: %w", ErrSentinel)
}

// main function
func main() {
    err1 := WrappedErrFromInternalCall()
    if xerrors.Is(err1, ErrSentinel) {
        fmt.Printf("%v\n", err1)
    }

    err2 := ErrFromInternalCall()
    if xerrors.Is(err2 , ErrSentinel) {
        fmt.Printf("%v\n", err2 )
    }
}
```