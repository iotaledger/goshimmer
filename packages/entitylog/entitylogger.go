package entitylog

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region EntityLog /////////////////////////////////////////////////////////////////////////////////////////////////

// EntityLog represents a log that allows to store entity specific logs. It is mainly used for debugging purposes and to
// monitor the changes made to different entities in the system.
type EntityLog struct {
	logEntryStorage       *objectstorage.ObjectStorage
	latestLogEntryID      logEntryID
	latestLogEntryIDMutex sync.Mutex
	loggerFactories       map[entityTypeID]LoggerFactory
	loggerFactoriesMutex  sync.RWMutex
}

// New creates a new EntityLog instance.
func New(store kvstore.KVStore) (entityLogger *EntityLog) {
	latestLogEntryID, err := logEntryIDFromStorage(store)
	if err != nil {
		panic(err)
	}

	entityLogger = &EntityLog{
		latestLogEntryID: latestLogEntryID,
		loggerFactories:  make(map[entityTypeID]LoggerFactory),
	}
	entityLogger.logEntryStorage = objectstorage.New(store.WithRealm([]byte{database.PrefixEntityLogger}), entityLogger.unmarshalEntityLogEntry, entityLogEntryPartitionKeys, objectstorage.CacheTime(0))

	return entityLogger
}

// RegisterEntity registers a new entity that can be used to log entity specific events.
func (e *EntityLog) RegisterEntity(entityName string, loggerFactory LoggerFactory) {
	e.loggerFactoriesMutex.Lock()
	defer e.loggerFactoriesMutex.Unlock()

	e.loggerFactories[newEntityTypeID(entityName)] = loggerFactory
}

// StoreLogEntry stores a new LogEntry in the EntityLog.
func (e *EntityLog) StoreLogEntry(logEntry LogEntry) {
	cachedObject, stored := e.logEntryStorage.StoreIfAbsent(&entityLogEntry{
		entityTypeID: newEntityTypeID(logEntry.EntityName()),
		entityID:     newEntityID(logEntry.EntityID()),
		logEntryID:   e.nextLogEntryID(),
		LogEntry:     logEntry,
	})
	if stored {
		cachedObject.Release()
	}
}

// LogEntries returns a set of CachedLogEntries that are associated with the given entityName (and optional entityID).
func (e *EntityLog) LogEntries(entityName string, entityID ...marshalutil.SimpleBinaryMarshaler) (cachedLogEntries CachedLogEntries) {
	hashedLogEntityType := blake2b.Sum256([]byte(entityName))
	var iterationPrefix []byte
	if len(entityID) >= 1 {
		hashedEntityID := blake2b.Sum256(entityID[0].Bytes())
		iterationPrefix = byteutils.ConcatBytes(hashedLogEntityType[:], hashedEntityID[:])
	} else {
		iterationPrefix = hashedLogEntityType[:]
	}

	cachedLogEntries = make(CachedLogEntries, 0)
	e.logEntryStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedLogEntries = append(cachedLogEntries, &cachedEntityLogEntry{CachedObject: cachedObject})
		return true
	}, objectstorage.WithIteratorPrefix(iterationPrefix))

	return
}

// Logger returns a Logger for the given entityName (and optional entityID).
func (e *EntityLog) Logger(entityName string, entityID ...marshalutil.SimpleBinaryMarshaler) Logger {
	return e.loggerFactories[newEntityTypeID(entityName)](e, entityID...)
}

// Shutdown shuts down the EntityLog and persist the collected LogEntries to the disk.
func (e *EntityLog) Shutdown() {
	e.logEntryStorage.Shutdown()
}

// unmarshalEntityLogEntry is the factory method for the object storage that takes care of correctly unmarshaling the different log entries.
func (e *EntityLog) unmarshalEntityLogEntry(key, data []byte) (logEntry objectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(byteutils.ConcatBytes(key, data))

	result := &entityLogEntry{}
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
	result.logEntryID = logEntryID(logEntryIDBytes)

	if result.LogEntry, err = e.loggerFactories[result.entityTypeID](e).UnmarshalLogEntry(marshalUtil.ReadRemainingBytes()); err != nil {
		return nil, err
	}

	return result, nil

}

// nextLogEntryID returns a new unique logEntryID.
func (e *EntityLog) nextLogEntryID() logEntryID {
	e.latestLogEntryIDMutex.Lock()
	defer e.latestLogEntryIDMutex.Unlock()

	e.latestLogEntryID++

	return e.latestLogEntryID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Logger ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Logger represents the interface for the Logger of a specific entity type.
type Logger interface {
	// LogDebug writes a log message with a LogLevel of Debug.
	LogDebug(args ...interface{})

	// LogDebugf formats and writes a log message with a LogLevel of Debug.
	LogDebugf(format string, args ...interface{})

	// LogInfo writes a log message with a LogLevel of Info.
	LogInfo(args ...interface{})

	// LogInfof formats and writes a log message with a LogLevel of Info.
	LogInfof(format string, args ...interface{})

	// LogWarn writes a log message with a LogLevel of Warn.
	LogWarn(args ...interface{})

	// LogWarnf formats and writes a log message with a LogLevel of Warn.
	LogWarnf(format string, args ...interface{})

	// LogError writes a log message with a LogLevel of Error.
	LogError(args ...interface{})

	// LogErrorf formats and writes a log message with a LogLevel of Error.
	LogErrorf(format string, args ...interface{})

	// Entries return a slice of LogEntries that have been stored in the Logger.
	Entries() []LogEntry

	// UnmarshalLogEntry unmarshals a LogEntry from a sequence of bytes.
	UnmarshalLogEntry(data []byte) (logEntry LogEntry, err error)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LogEntry /////////////////////////////////////////////////////////////////////////////////////////////////////

type LogEntry interface {
	EntityName() string

	EntityID() marshalutil.SimpleBinaryMarshaler

	LogLevel() LogLevel

	Time() time.Time

	Message() string

	String() string

	marshalutil.SimpleBinaryMarshaler
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LogLevel /////////////////////////////////////////////////////////////////////////////////////////////////////

// LogLevel defines different logging severity levels.
type LogLevel uint8

const (
	// Debug represents logs that are used for interactive investigation during development. These logs should primarily
	// contain information useful for debugging and have no long-term value.
	Debug LogLevel = iota

	// Info represents logs that track the general flow of the application. These logs should have long-term value.
	Info

	// Warn represents logs that highlight an abnormal or unexpected event in the application flow, but do not otherwise
	// cause the application execution to stop.
	Warn

	// Error represents logs that highlight when the current flow of execution is stopped due to a failure. These should
	// indicate a failure in the current activity, not an application-wide failure.
	Error
)

// LogLevelFromMarshalUtil unmarshals a LogLevel using a MarshalUtil (for easier unmarshaling).
func LogLevelFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (logLevel LogLevel, err error) {
	logLevelByte, err := marshalUtil.ReadByte()
	if err != nil {
		return 0, err
	}
	logLevel = LogLevel(logLevelByte)

	return
}

// String returns a human-readable version of the LogLevel.
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

// Bytes returns a marshaled version of the LogLevel.
func (l LogLevel) Bytes() []byte {
	return []byte{byte(l)}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LoggerFactory ////////////////////////////////////////////////////////////////////////////////////////////////

// LoggerFactory represents the interface for creating new generic Loggers.
type LoggerFactory func(entityLogger *EntityLog, entityID ...marshalutil.SimpleBinaryMarshaler) Logger

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedLogEntries /////////////////////////////////////////////////////////////////////////////////////////////

// CachedLogEntries defines a slice of *cachedEntityLogEntry.
type CachedLogEntries []*cachedEntityLogEntry

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedLogEntries) Unwrap() (unwrappedLogEntries []LogEntry) {
	unwrappedLogEntries = make([]LogEntry, len(c))
	for i, cachedEntityLogEntry := range c {
		untypedObject := cachedEntityLogEntry.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*entityLogEntry)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedLogEntries[i] = typedObject.LogEntry
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedLogEntries) Consume(consumer func(logEntry LogEntry), forceRelease ...bool) (consumed bool) {
	for _, cachedEntityLogEntry := range c {
		consumed = cachedEntityLogEntry.Consume(func(logEntry *entityLogEntry) {
			consumer(logEntry.LogEntry)
		}, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedLogEntries) Release(force ...bool) {
	for _, cachedEntityLogEntry := range c {
		cachedEntityLogEntry.Release(force...)
	}
}

// String returns a human-readable version of the CachedLogEntries.
func (c CachedLogEntries) String() string {
	structBuilder := stringify.StructBuilder("CachedLogEntries")
	for i, cachedEntityLogEntry := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedEntityLogEntry))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region internal types ///////////////////////////////////////////////////////////////////////////////////////////////

// region entityTypeID

type entityTypeID [32]byte

func newEntityTypeID(entityName string) entityTypeID {
	return blake2b.Sum256([]byte(entityName))
}

func (e entityTypeID) String() string {
	return "entityTypeID(" + base58.Encode(e[:]) + ")"
}

// endregion

// region entityID

type entityID [32]byte

func newEntityID(entityID marshalutil.SimpleBinaryMarshaler) entityID {
	return blake2b.Sum256(entityID.Bytes())
}

func (e entityID) String() string {
	return "entityID(" + base58.Encode(e[:]) + ")"
}

// endregion

// region logEntryID

// logEntryID represents a unique identifier for log entries in the EntityLog.
type logEntryID uint64

// logEntryIDFromStorage unmarshals a logEntryID from a KVStore.
func logEntryIDFromStorage(store kvstore.KVStore, optionalKey ...kvstore.Key) (logEntryID logEntryID, err error) {
	key := kvstore.Key("logEntryID")
	if len(optionalKey) != 0 {
		key = optionalKey[0]
	}

	storedLatestLogEntryID, err := store.Get(key)
	if err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		return 0, errors.Errorf("failed to load logEntryID from KVStore: %w", err)
	}
	if storedLatestLogEntryID != nil {
		if logEntryID, _, err = logEntryIDFromBytes(storedLatestLogEntryID); err != nil {
			return 0, errors.Errorf("failed to parse logEntryID from bytes: %w", err)
		}
	}

	return logEntryID, nil
}

// logEntryIDFromBytes unmarshals a logEntryID from a sequence of bytes.
func logEntryIDFromBytes(sequenceIDBytes []byte) (logEntryID logEntryID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceIDBytes)
	if logEntryID, err = logEntryIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse logEntryID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// logEntryIDFromMarshalUtil unmarshals a logEntryID using a MarshalUtil (for easier unmarshaling).
func logEntryIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (l logEntryID, err error) {
	untypedLogEntryID, err := marshalUtil.ReadUint64()
	if err != nil {
		return 0, errors.Errorf("failed to parse logEntryID from MarshalUtil (%v): %w", err, cerrors.ErrParseBytesFailed)
	}

	return logEntryID(untypedLogEntryID), nil
}

// String returns a human-readable version of the logEntryID.
func (l logEntryID) String() string {
	return fmt.Sprintf("logEntryID(%d)", uint64(l))
}

// endregion

// region entityLogEntry

// entityLogEntryPartitionKeys defines the "layout" of the key. This enables prefix iterations in the objectstorage.
var entityLogEntryPartitionKeys = objectstorage.PartitionKey([]int{32, 32, marshalutil.Uint64Size}...)

type entityLogEntry struct {
	entityTypeID entityTypeID
	entityID     entityID
	logEntryID   logEntryID
	LogEntry     LogEntry

	objectstorage.StorableObjectFlags
}

func (w *entityLogEntry) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

func (w *entityLogEntry) ObjectStorageKey() []byte {
	return marshalutil.New().
		WriteBytes(w.entityTypeID[:]).
		WriteBytes(w.entityID[:]).
		WriteUint64(uint64(w.logEntryID)).
		Write(w.LogEntry).
		Bytes()
}

func (w *entityLogEntry) ObjectStorageValue() []byte {
	return w.LogEntry.Bytes()
}

func (w *entityLogEntry) String() string {
	return stringify.Struct("entityLogEntry",
		stringify.StructField("entityTypeID", w.entityTypeID),
		stringify.StructField("entityID", w.entityID),
		stringify.StructField("logEntryID", w.logEntryID),
		stringify.StructField("LogEntry", w.LogEntry),
	)
}

var _ objectstorage.StorableObject = &entityLogEntry{}

// endregion

// region cachedEntityLogEntry

// cachedEntityLogEntry is a wrapper for a stored cached object representing an approver.
type cachedEntityLogEntry struct {
	objectstorage.CachedObject
}

// Unwrap unwraps the cached approver into the underlying approver.
// If stored object cannot be cast into an approver or has been deleted, it returns nil.
func (c *cachedEntityLogEntry) Unwrap() *entityLogEntry {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*entityLogEntry)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume consumes the cachedEntityLogEntry.
// It releases the object when the callback is done.
// It returns true if the callback was called.
func (c *cachedEntityLogEntry) Consume(consumer func(approver *entityLogEntry), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*entityLogEntry))
	}, forceRelease...)
}

// endregion

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
