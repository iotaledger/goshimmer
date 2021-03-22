# Code guidelines
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