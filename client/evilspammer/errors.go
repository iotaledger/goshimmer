package evilspammer

import (
	"fmt"
	"github.com/cockroachdb/errors"
	"go.uber.org/atomic"
	"sync"
)

var (
	ErrFailPostTransaction = errors.New("failed to post transaction")
	ErrFailSendDataMessage = errors.New("failed to send a data message")
	ErrTransactionIsNil    = errors.New("provided transaction is nil")
	ErrFailToPrepareBatch  = errors.New("custom conflict batch could not be prepared")
	ErrInsufficientClients = errors.New("insufficient clients to send conflicts")
	ErrInputsNotSolid      = errors.New("not all inputs are solid")
)

// ErrorCounter counts errors that appeared during the spam,
// as during the spam they are ignored and allows to print the summary (might be useful for debugging).
type ErrorCounter struct {
	errorsMap       map[error]*atomic.Int64
	errInTotalCount *atomic.Int64
	mutex           sync.RWMutex
}

func NewErrorCount() *ErrorCounter {
	e := &ErrorCounter{
		errorsMap:       make(map[error]*atomic.Int64),
		errInTotalCount: atomic.NewInt64(0),
	}
	return e
}

func (e *ErrorCounter) CountError(err error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// check if error is already in the map
	if _, ok := e.errorsMap[err]; !ok {
		e.errorsMap[err] = atomic.NewInt64(0)
	}
	e.errInTotalCount.Add(1)
	e.errorsMap[err].Add(1)
}

func (e *ErrorCounter) GetTotalErrorCount() int64 {
	return e.errInTotalCount.Load()
}

func (e *ErrorCounter) GetErrorsSummary() string {
	if len(e.errorsMap) == 0 {
		return "No errors encountered"
	}
	msg := "Errors encountered during spam:\n"
	for key, value := range e.errorsMap {
		msg += fmt.Sprintf("%s: %d\n", key.Error(), value.Load())
	}
	return msg
}
