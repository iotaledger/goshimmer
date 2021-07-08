package ledgerstate

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/entitylogger"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/typeutils"
)

const BranchEntityName = "Branch"

// region BranchLogger /////////////////////////////////////////////////////////////////////////////////////////////////

type BranchLogger struct {
	entityLogger *entitylogger.EntityLogger
	branchID     BranchID
}

func NewBranchLogger(entityLogger *entitylogger.EntityLogger, entityID ...marshalutil.SimpleBinaryMarshaler) entitylogger.Logger {
	var branchID BranchID
	if len(entityID) >= 1 {
		branchID = entityID[0].(BranchID)
	}

	return &BranchLogger{
		entityLogger: entityLogger,
		branchID:     branchID,
	}
}

func (b *BranchLogger) Entries() (entries []entitylogger.LogEntry) {
	if b.branchID == UndefinedBranchID {
		b.entityLogger.LogEntries(BranchEntityName).Consume(func(wrappedLogEntry *entitylogger.EntityLogEntry) {
			entries = append(entries, wrappedLogEntry.LogEntry)
		})
	} else {
		b.entityLogger.LogEntries(BranchEntityName, b.branchID).Consume(func(wrappedLogEntry *entitylogger.EntityLogEntry) {
			entries = append(entries, wrappedLogEntry.LogEntry)
		})
	}

	return
}

func (b *BranchLogger) LogDebug(args ...interface{}) {
	b.storeLogEntry(entitylogger.Debug, args...)
}

func (b *BranchLogger) LogDebugf(format string, args ...interface{}) {
	b.storeLogEntryF(entitylogger.Debug, format, args...)
}

func (b *BranchLogger) LogInfo(args ...interface{}) {
	b.storeLogEntry(entitylogger.Info, args...)
}

func (b *BranchLogger) LogInfof(format string, args ...interface{}) {
	b.storeLogEntryF(entitylogger.Info, format, args...)
}

func (b *BranchLogger) LogWarn(args ...interface{}) {
	b.storeLogEntry(entitylogger.Warn, args...)
}

func (b *BranchLogger) LogWarnf(format string, args ...interface{}) {
	b.storeLogEntryF(entitylogger.Warn, format, args...)
}

func (b *BranchLogger) LogError(args ...interface{}) {
	b.storeLogEntry(entitylogger.Error, args...)
}

func (b *BranchLogger) LogErrorf(format string, args ...interface{}) {
	b.storeLogEntryF(entitylogger.Error, format, args...)
}

func (b *BranchLogger) UnmarshalLogEntry(data []byte) (logEntry entitylogger.LogEntry, err error) {
	marshalUtil := marshalutil.New(data)

	branchLogEntry := &BranchLogEntry{}
	if branchLogEntry.branchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		return nil, err
	}
	if branchLogEntry.time, err = marshalUtil.ReadTime(); err != nil {
		return nil, err
	}
	if branchLogEntry.logLevel, err = entitylogger.LogLevelFromMarshalUtil(marshalUtil); err != nil {
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

func (b *BranchLogger) storeLogEntry(logLevel entitylogger.LogLevel, args ...interface{}) {
	b.entityLogger.StoreLogEntry(NewBranchLogEntry(b.branchID, logLevel, fmt.Sprint(args...)))
}

func (b *BranchLogger) storeLogEntryF(logLevel entitylogger.LogLevel, format string, args ...interface{}) {
	b.entityLogger.StoreLogEntry(NewBranchLogEntry(b.branchID, logLevel, fmt.Sprintf(format, args...)))
}

var _ entitylogger.Logger = &BranchLogger{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchLogEntry ///////////////////////////////////////////////////////////////////////////////////////////////

// BranchLogEntry represents a log entry related to Branches in the ledger state.
type BranchLogEntry struct {
	branchID BranchID
	logLevel entitylogger.LogLevel
	time     time.Time
	message  string

	objectstorage.StorableObjectFlags
}

// NewBranchLogEntry returns a new log entry that contains Branch related information.
func NewBranchLogEntry(branchID BranchID, logLevel entitylogger.LogLevel, message string) *BranchLogEntry {
	return &BranchLogEntry{
		branchID: branchID,
		logLevel: logLevel,
		time:     time.Now(),
		message:  message,
	}
}

func (b *BranchLogEntry) EntityName() string {
	return BranchEntityName
}

func (b *BranchLogEntry) EntityID() marshalutil.SimpleBinaryMarshaler {
	return b.branchID
}

func (b *BranchLogEntry) LogLevel() entitylogger.LogLevel {
	return b.logLevel
}

// BranchID returns the identifier of the Branch that this log entry belongs to.
func (b *BranchLogEntry) BranchID() BranchID {
	return b.branchID
}

// Time returns the time when the log entry was created.
func (b *BranchLogEntry) Time() time.Time {
	return b.time
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
		stringify.StructField("LogLevel", b.LogLevel()),
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
var _ entitylogger.LogEntry = &BranchLogEntry{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
