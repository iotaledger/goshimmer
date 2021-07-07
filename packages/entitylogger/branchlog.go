package entitylogger

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/typeutils"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

const BranchEntityName = "Branch"

// region BranchLogger /////////////////////////////////////////////////////////////////////////////////////////////////

type BranchLogger struct {
	entityLogger *EntityLogger
	branchID     ledgerstate.BranchID
}

func NewBranchLogger(entityLogger *EntityLogger, entityID ...marshalutil.SimpleBinaryMarshaler) Logger {
	var branchID ledgerstate.BranchID
	if len(entityID) >= 1 {
		branchID = entityID[0].(ledgerstate.BranchID)
	}

	return &BranchLogger{
		entityLogger: entityLogger,
		branchID:     branchID,
	}
}

func (b *BranchLogger) Entries() (entries []LogEntry) {
	if b.branchID == ledgerstate.UndefinedBranchID {
		b.entityLogger.LogEntries(BranchEntityName).Consume(func(wrappedLogEntry *WrappedLogEntry) {
			entries = append(entries, wrappedLogEntry.logEntry)
		})
	} else {
		b.entityLogger.LogEntries(BranchEntityName, b.branchID).Consume(func(wrappedLogEntry *WrappedLogEntry) {
			entries = append(entries, wrappedLogEntry.logEntry)
		})
	}

	return
}

func (b *BranchLogger) LogDebug(args ...interface{}) {
	b.storeLogEntry(Debug, args...)
}

func (b *BranchLogger) LogDebugf(format string, args ...interface{}) {
	b.storeLogEntryF(Debug, format, args...)
}

func (b *BranchLogger) LogInfo(args ...interface{}) {
	b.storeLogEntry(Info, args...)
}

func (b *BranchLogger) LogInfof(format string, args ...interface{}) {
	b.storeLogEntryF(Info, format, args...)
}

func (b *BranchLogger) LogWarn(args ...interface{}) {
	b.storeLogEntry(Warn, args...)
}

func (b *BranchLogger) LogWarnf(format string, args ...interface{}) {
	b.storeLogEntryF(Warn, format, args...)
}

func (b *BranchLogger) LogError(args ...interface{}) {
	b.storeLogEntry(Error, args...)
}

func (b *BranchLogger) LogErrorf(format string, args ...interface{}) {
	b.storeLogEntryF(Error, format, args...)
}

func (b *BranchLogger) UnmarshalLogEntry(data []byte) (logEntry LogEntry, err error) {
	marshalUtil := marshalutil.New(data)

	branchLogEntry := &BranchLogEntry{}
	if branchLogEntry.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		return nil, err
	}
	if branchLogEntry.time, err = marshalUtil.ReadTime(); err != nil {
		return nil, err
	}
	if branchLogEntry.logLevel, err = LogLevelFromMarshalUtil(marshalUtil); err != nil {
		return nil, err
	}
	messageLength, err := marshalUtil.ReadUint64()
	if err != nil {
		return nil, err
	}
	messageBytes, err := marshalUtil.ReadBytes(int(messageLength))
	branchLogEntry.message = typeutils.BytesToString(messageBytes)

	return branchLogEntry, nil
}

func (b *BranchLogger) storeLogEntry(logLevel LogLevel, args ...interface{}) {
	b.entityLogger.StoreLogEntry(NewBranchLogEntry(b.branchID, logLevel, fmt.Sprint(args...)))
}

func (b *BranchLogger) storeLogEntryF(logLevel LogLevel, format string, args ...interface{}) {
	b.entityLogger.StoreLogEntry(NewBranchLogEntry(b.branchID, logLevel, fmt.Sprintf(format, args...)))
}

var _ Logger = &BranchLogger{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchLogEntry ///////////////////////////////////////////////////////////////////////////////////////////////

// BranchLogEntry represents a log entry related to Branches in the ledger state.
type BranchLogEntry struct {
	branchID ledgerstate.BranchID
	logLevel LogLevel
	time     time.Time
	message  string

	objectstorage.StorableObjectFlags
}

// NewBranchLogEntry returns a new log entry that contains Branch related information.
func NewBranchLogEntry(branchID ledgerstate.BranchID, logLevel LogLevel, message string) *BranchLogEntry {
	return &BranchLogEntry{
		branchID: branchID,
		logLevel: logLevel,
		time:     time.Now(),
		message:  message,
	}
}

func (b *BranchLogEntry) Entity() string {
	return BranchEntityName
}

func (b *BranchLogEntry) EntityID() marshalutil.SimpleBinaryMarshaler {
	return b.branchID
}

func (b *BranchLogEntry) LogLevel() LogLevel {
	return b.logLevel
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

func (b *BranchLogEntry) Bytes() []byte {
	messageBytes := typeutils.StringToBytes(b.Message())

	return marshalutil.New().
		Write(b.branchID).
		WriteTime(b.time).
		Write(b.logLevel).
		WriteUint64(uint64(len(messageBytes))).
		WriteBytes(messageBytes).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ LogEntry = &BranchLogEntry{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
