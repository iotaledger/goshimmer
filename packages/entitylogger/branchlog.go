package entitylogger

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region BranchLogger /////////////////////////////////////////////////////////////////////////////////////////////////

type BranchLogger struct {
	entityLogger *EntityLogger
	branchID     ledgerstate.BranchID
}

func (b *BranchLogger) Entries() []LogEntry {
	panic("implement me")
}

func (b *BranchLogger) LogDebug(args ...interface{}) {
	b.storeLogEntry(Debug, args...)
}

func (b *BranchLogger) LogDebugf(format string, args ...interface{}) {
	b.storeLogEntryF(Debug, format, args...)
}

func (b BranchLogger) LogInfo(args ...interface{}) {
	b.storeLogEntry(Info, args...)
}

func (b BranchLogger) LogInfof(format string, args ...interface{}) {
	b.storeLogEntryF(Info, format, args...)
}

func (b BranchLogger) LogWarn(args ...interface{}) {
	b.storeLogEntry(Warn, args...)
}

func (b BranchLogger) LogWarnf(format string, args ...interface{}) {
	b.storeLogEntryF(Warn, format, args...)
}

func (b *BranchLogger) LogError(args ...interface{}) {
	b.storeLogEntry(Error, args...)
}

func (b BranchLogger) LogErrorf(format string, args ...interface{}) {
	b.storeLogEntryF(Error, format, args...)
}

func (b *BranchLogger) storeLogEntry(logLevel LogLevel, args ...interface{}) {
	b.entityLogger.StoreLogEntry(NewBranchLogEntry(b.branchID, logLevel, b.entityLogger.LogEntryID(), fmt.Sprint(args...)))
}

func (b *BranchLogger) storeLogEntryF(logLevel LogLevel, format string, args ...interface{}) {
	b.entityLogger.StoreLogEntry(NewBranchLogEntry(b.branchID, logLevel, b.entityLogger.LogEntryID(), fmt.Sprintf(format, args...)))
}

var _ Logger = &BranchLogger{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchLogEntry ///////////////////////////////////////////////////////////////////////////////////////////////

// BranchLogEntry represents a log entry related to Branches in the ledger state.
type BranchLogEntry struct {
	branchID   ledgerstate.BranchID
	logLevel   LogLevel
	logEntryID LogEntryID
	time       time.Time
	message    string

	objectstorage.StorableObjectFlags
}

// NewBranchLogEntry returns a new log entry that contains Branch related information.
func NewBranchLogEntry(branchID ledgerstate.BranchID, logLevel LogLevel, logEntryID LogEntryID, message string) *BranchLogEntry {
	return &BranchLogEntry{
		branchID:   branchID,
		logLevel:   logLevel,
		logEntryID: logEntryID,
		time:       time.Now(),
		message:    message,
	}
}

func (b *BranchLogEntry) EntityID() [32]byte {
	return blake2b.Sum256(b.branchID[:])
}

func (b *BranchLogEntry) LogLevel() LogLevel {
	panic("implement me")
}

func (b *BranchLogEntry) LogEntryID() LogEntryID {
	return b.logEntryID
}

// BranchID returns the identifier of the Branch that this log entry belongs to.
func (b *BranchLogEntry) BranchID() ledgerstate.BranchID {
	return b.branchID
}

// Time returns the time when the log entry was created.
func (b *BranchLogEntry) Time() time.Time {
	return b.time
}

// Type returns the type of the log entry. This can be used to distinguish between different kinds of log entries like
// function calls, the creation of entities or updates of their properties.
func (b *BranchLogEntry) Level() LogLevel {
	return b.logLevel
}

// Message returns the message of the log entry.
func (b *BranchLogEntry) Message() string {
	return b.message
}

// String returns a human-readable version of the BranchLogEntry.
func (b *BranchLogEntry) String() string {
	return stringify.Struct("BranchLogEntry",
		stringify.StructField("BranchID", b.BranchID()),
		stringify.StructField("Time", b.Time()),
		stringify.StructField("Level", b.Level()),
		stringify.StructField("Message", b.Message()),
	)
}

func (b *BranchLogEntry) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

func (b *BranchLogEntry) ObjectStorageKey() []byte {
	panic("implement me")
}

func (b *BranchLogEntry) ObjectStorageValue() []byte {
	panic("implement me")
}

// code contract (make sure the struct implements all required methods)
var _ LogEntry = &BranchLogEntry{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
