package entitylogger

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region EntityLogger /////////////////////////////////////////////////////////////////////////////////////////////////

type EntityLogger struct {
	entityLogStorage      *objectstorage.ObjectStorage
	latestLogEntryID      LogEntryID
	latestLogEntryIDMutex sync.Mutex
	loggerFactories       map[EntityTypeID]LoggerFactory
	loggerFactoriesMutex  sync.RWMutex
}

func New(store kvstore.KVStore) (entityLogger *EntityLogger) {
	entityLogger = &EntityLogger{
		loggerFactories: make(map[EntityTypeID]LoggerFactory),
	}

	entityLogger.entityLogStorage = objectstorage.New(store.WithRealm([]byte{database.PrefixEntityLogger}), entityLogger.UnmarshalWrappedLogEntry, WrappedLogEntryPartitionKeys, objectstorage.CacheTime(0))

	return entityLogger
}

func (e *EntityLogger) UnmarshalWrappedLogEntry(key, data []byte) (wrappedLogEntry objectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(byteutils.ConcatBytes(key, data))

	result := &WrappedLogEntry{}
	entityTypeIDBytes, err := marshalUtil.ReadBytes(32)
	if err != nil {
		return nil, err
	}
	copy(result.entityTypeID[:], entityTypeIDBytes)

	entityIDBytes, err := marshalUtil.ReadBytes(32)
	if err != nil {
		return nil, err
	}
	copy(result.entityID[:], entityIDBytes)

	logEntryIDBytes, err := marshalUtil.ReadUint64()
	if err != nil {
		return nil, err
	}
	result.logEntryID = LogEntryID(logEntryIDBytes)

	if result.logEntry, err = e.loggerFactories[result.entityTypeID](e).UnmarshalLogEntry(marshalUtil.ReadRemainingBytes()); err != nil {
		return nil, err
	}

	return result, nil

}

func (e *EntityLogger) EntityTypeID(entityName string) EntityTypeID {
	return blake2b.Sum256([]byte(entityName))
}

func (e *EntityLogger) RegisterEntity(entityName string, loggerFactory LoggerFactory) {
	e.loggerFactoriesMutex.Lock()
	defer e.loggerFactoriesMutex.Unlock()

	e.loggerFactories[e.EntityTypeID(entityName)] = loggerFactory
}

func (e *EntityLogger) NewLogEntryID() LogEntryID {
	e.latestLogEntryIDMutex.Lock()
	defer e.latestLogEntryIDMutex.Unlock()

	e.latestLogEntryID++

	return e.latestLogEntryID
}

func (e *EntityLogger) StoreLogEntry(logEntry LogEntry) {
	cachedObject, stored := e.entityLogStorage.StoreIfAbsent(&WrappedLogEntry{
		entityTypeID: e.EntityTypeID(logEntry.Entity()),
		entityID:     e.EntityID(logEntry.EntityID()),
		logEntryID:   e.NewLogEntryID(),
		logEntry:     logEntry,
	})
	if stored {
		cachedObject.Release()
	}
}

func (e *EntityLogger) EntityID(entityID marshalutil.SimpleBinaryMarshaler) EntityID {
	return blake2b.Sum256(entityID.Bytes())
}

func (e *EntityLogger) LogEntries(entityName string, entityID ...marshalutil.SimpleBinaryMarshaler) (cachedLogEntries CachedWrappedLogEntries) {
	hashedLogEntityType := blake2b.Sum256([]byte(entityName))
	var iterationPrefix []byte
	if len(entityID) >= 1 {
		hashedEntityID := blake2b.Sum256(entityID[0].Bytes())
		iterationPrefix = byteutils.ConcatBytes(hashedLogEntityType[:], hashedEntityID[:])
	} else {
		iterationPrefix = hashedLogEntityType[:]
	}

	cachedLogEntries = make(CachedWrappedLogEntries, 0)
	e.entityLogStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedLogEntries = append(cachedLogEntries, &CachedWrappedLogEntry{CachedObject: cachedObject})
		return true
	}, objectstorage.WithIteratorPrefix(iterationPrefix))

	return
}

func (e *EntityLogger) Logger(entityName string, entityID ...marshalutil.SimpleBinaryMarshaler) Logger {
	return e.loggerFactories[e.EntityTypeID(entityName)](e, entityID...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Logger ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Logger interface {
	LogDebug(args ...interface{})

	LogDebugf(format string, args ...interface{})

	LogInfo(args ...interface{})

	LogInfof(format string, args ...interface{})

	LogWarn(args ...interface{})

	LogWarnf(format string, args ...interface{})

	LogError(args ...interface{})

	LogErrorf(format string, args ...interface{})

	Entries() []LogEntry

	UnmarshalLogEntry(data []byte) (logEntry LogEntry, err error)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LogEntry /////////////////////////////////////////////////////////////////////////////////////////////////////

type LogEntry interface {
	Entity() string
	EntityID() marshalutil.SimpleBinaryMarshaler
	LogLevel() LogLevel
	Time() time.Time
	Message() string
	String() string

	marshalutil.SimpleBinaryMarshaler
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region WrappedLogEntry //////////////////////////////////////////////////////////////////////////////////////////////

// WrappedLogEntryPartitionKeys defines the "layout" of the key. This enables prefix iterations in the objectstorage.
var WrappedLogEntryPartitionKeys = objectstorage.PartitionKey([]int{32, 32, marshalutil.Uint64Size}...)

type WrappedLogEntry struct {
	entityTypeID EntityTypeID
	entityID     EntityID
	logEntryID   LogEntryID
	logEntry     LogEntry

	objectstorage.StorableObjectFlags
}

func (w *WrappedLogEntry) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

func (w *WrappedLogEntry) ObjectStorageKey() []byte {
	return marshalutil.New().
		WriteBytes(w.entityTypeID[:]).
		WriteBytes(w.entityID[:]).
		WriteUint64(uint64(w.logEntryID)).
		Write(w.logEntry).
		Bytes()
}

func (w *WrappedLogEntry) ObjectStorageValue() []byte {
	return w.logEntry.Bytes()
}

func (w *WrappedLogEntry) String() string {
	return stringify.Struct("WrappedLogEntry",
		stringify.StructField("EntityTypeID", w.entityTypeID),
		stringify.StructField("EntityID", w.entityID),
		stringify.StructField("LogEntryID", w.logEntryID),
		stringify.StructField("LogEntry", w.logEntry),
	)
}

var _ objectstorage.StorableObject = &WrappedLogEntry{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedWrappedLogEntry ////////////////////////////////////////////////////////////////////////////////////////

// CachedWrappedLogEntry is a wrapper for a stored cached object representing an approver.
type CachedWrappedLogEntry struct {
	objectstorage.CachedObject
}

// Unwrap unwraps the cached approver into the underlying approver.
// If stored object cannot be cast into an approver or has been deleted, it returns nil.
func (c *CachedWrappedLogEntry) Unwrap() *WrappedLogEntry {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*WrappedLogEntry)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume consumes the cachedWrappedLogEntry.
// It releases the object when the callback is done.
// It returns true if the callback was called.
func (c *CachedWrappedLogEntry) Consume(consumer func(approver *WrappedLogEntry), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*WrappedLogEntry))
	}, forceRelease...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedWrappedLogEntries //////////////////////////////////////////////////////////////////////////////////////

// CachedWrappedLogEntries defines a slice of *CachedWrappedLogEntry.
type CachedWrappedLogEntries []*CachedWrappedLogEntry

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedWrappedLogEntries) Unwrap() (unwrappedWrappedLogEntries []*WrappedLogEntry) {
	unwrappedWrappedLogEntries = make([]*WrappedLogEntry, len(c))
	for i, cachedWrappedLogEntry := range c {
		untypedObject := cachedWrappedLogEntry.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*WrappedLogEntry)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedWrappedLogEntries[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedWrappedLogEntries) Consume(consumer func(approver *WrappedLogEntry), forceRelease ...bool) (consumed bool) {
	for _, cachedWrappedLogEntry := range c {
		consumed = cachedWrappedLogEntry.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedWrappedLogEntries) Release(force ...bool) {
	for _, cachedWrappedLogEntry := range c {
		cachedWrappedLogEntry.Release(force...)
	}
}

// String returns a human-readable version of the CachedWrappedLogEntries.
func (c CachedWrappedLogEntries) String() string {
	structBuilder := stringify.StructBuilder("CachedWrappedLogEntries")
	for i, cachedWrappedLogEntry := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedWrappedLogEntry))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EntityTypeID /////////////////////////////////////////////////////////////////////////////////////////////////

type EntityTypeID [32]byte

func (e EntityTypeID) String() string {
	return "EntityTypeID(" + base58.Encode(e[:]) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EntityID /////////////////////////////////////////////////////////////////////////////////////////////////////

type EntityID [32]byte

func (e EntityID) String() string {
	return "EntityID(" + base58.Encode(e[:]) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LogEntryID ///////////////////////////////////////////////////////////////////////////////////////////////////

type LogEntryID uint64

func (l LogEntryID) String() string {
	return fmt.Sprintf("LogEntryID(%d)", uint64(l))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LogLevel /////////////////////////////////////////////////////////////////////////////////////////////////////

type LogLevel uint8

const (
	Debug LogLevel = iota

	Info

	Warn

	Error
)

func LogLevelFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (logLevel LogLevel, err error) {
	logLevelByte, err := marshalUtil.ReadByte()
	if err != nil {
		return 0, err
	}
	logLevel = LogLevel(logLevelByte)

	return
}

func (l LogLevel) String() string {
	switch l {
	case Debug:
		return "LogLevel(Debug)"
	case Info:
		return "LogLevel(Info)"
	case Warn:
		return "LogLevel(Warn)"
	case Error:
		return "LogLevel(Warn)"
	default:
		return "LogLevel(Unknown=" + strconv.Itoa(int(l)) + ")"
	}
}

func (l LogLevel) Bytes() []byte {
	return []byte{byte(l)}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LoggerFactory ////////////////////////////////////////////////////////////////////////////////////////////////

type LoggerFactory func(entityLogger *EntityLogger, entityID ...marshalutil.SimpleBinaryMarshaler) Logger

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
