# Code guidelines

## General guidelines

- Do not use `log.Fatal()`, or `os.Exit()` outside of the main. It immediately terminates the program, all defers are ignored and no graceful shutdown is possible. 
    As it can lead to inconsistencies, you should 
  propagate the error up to the main, and let the main function exit instead. 
  
    You should avoid panics as well, almost always use errors. [Example](https://github.com/iotaledger/goshimmer/blob/f75ce47eeaa3bf930b368754ac24b72f768a5964/plugins/autopeering/autopeering.go#L135).
  
- Don’t duplicate code, reuse it. This also applies to tests. Example: [duplicate1](https://github.com/iotaledger/goshimmer/blob/f75ce47eeaa3bf930b368754ac24b72f768a5964/packages/ledgerstate/branch_dag.go#L969) and [duplicate2](https://github.com/iotaledger/goshimmer/blob/f75ce47eeaa3bf930b368754ac24b72f768a5964/packages/ledgerstate/branch_dag.go#L1053)
  
- Unhandled errors can cause bugs, and make it harder to diagnose problems. Try to handle all errors: propagate them to the caller or log them. 
    Even if the function call is used with a defer, and it is inconvenient to handle the error it returns, still handle it. Wrap the function call in an anonymous function assign error to the upper error as in the following example:
  
```go
    defer func() {
        cerr := f.Close()
        if err == nil {
            err = errors.Wrap(cerr, "failed to close file")
        }
    }()
```

- Wrap errors with `errors.Wrap()` when returning them to the caller. It adds the stack trace and a custom message to the error. Without that information investigating an issue is very hard.
  
- Use `errors.Is()` instead of direct error comparison. This function unwraps errors recursively. [Example](https://github.com/iotaledger/goshimmer/pull/1113/files#diff-05fdc081489a8d5a61224d812f9bbd7bc77edf9769ed00d95ea024d2a44a699aL62).

- Propagate `ctx` and use APIs that accept `ctx`, start exposing APIs that accept `ctx`. Context is a native way for timeouts/cancellation in Go. It allows writing more resilient and fault tolerant code. [Example](https://github.com/iotaledger/goshimmer/pull/1113/files#diff-f2820ed0d3d4d9ea05b78b1dd3978dbcf9401c8caaa8cc40cc1c0342a55379fcL35).

- Do not shadow builtin functions like copy, len, new etc. [Example](https://github.com/iotaledger/goshimmer/pull/1113/files#diff-f07268750a44da26386469c1b1e93574a678c3d418fce9e1f186d5f1991a92eaL14).

- Do not shadow imported packages. [Example](https://github.com/iotaledger/goshimmer/blob/f75ce47eeaa3bf930b368754ac24b72f768a5964/plugins/webapi/value/sendtransactionbyjson.go#L172).

- Do not do `[:]` on a slice. It has no effect. [Example](https://github.com/iotaledger/goshimmer/pull/1113/files#diff-299a1ac5fa09739ea07b7c806ee2785d83eea110f8af143dbc853a25e4819116L133).

- Avoid naked returns if the function isn’t very small. It makes the code more readable.

- Define explicit constants for strings that are used 3 times or more. It makes the code more maintainable.

- Define explicit constants for all numbers. It makes the code more readable.

- Do not write really long and complex functions. Split them into smaller ones.

- Treat comments as regular text/documentation. Start with a capital letter, set space after `//`, and end them with a dot. It is a good habit since Go package docs are generated automatically from the comments, and displayed on the godoc site.

## Error Handling

We use the new error wrapping API and behavior introduced with Go 1.13. However, we use the [github.com/cockroachdb/errors](http://github.com/cockroachdb/errors) drop-in replacement, which follows the Go 2 design draft, and enables us to have a stack trace for every "wrapping" of the error.

Errors should always be wrapped and annotated with an additional message at each step. The following example shows how errors are wrapped and turned into the corresponding sentinel errors.

```go
package example

import (
    "3rdPartyLibrary"

    "github.com/cockroachdb/errors"
)

// define error variables to make errors identifiable (sentinel errors)
var ErrSentinel = errors.New("identifiable error")

// turn anonymous 3rd party errors into identifiable ones
func SentinelErrFrom3rdParty() (result interface{}, err error)
    if result, err = 3rdPartyLibrary.DoSomething(); err != nil {
        err = errors.Errorf("failed to do something (%v): %w", err, ErrSentinel)
        return
    }

    return
}

// wrap recursive errors at each step
func WrappedErrFromInternalCall() error {
    return errors.Errorf("wrapped internal error: %w", SentinelErrFrom3rdParty())
}

// create "new" identifiable internal errors that are not originating in 3rd party libs
func ErrFromInternalCall() error {
    return errors.Errorf("internal error: %w", ErrSentinel)
}

// main function
func main() {
    err1 := WrappedErrFromInternalCall()
    if errors.Is(err1, ErrSentinel) {
        fmt.Printf("%v\n", err1)
    }

    err2 := ErrFromInternalCall()
    if errors.Is(err2 , ErrSentinel) {
        fmt.Printf("%v\n", err2 )
    }
}
```
