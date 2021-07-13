# Object storage

In GoShimmer the `ObjectStorage` is used as a base data structure for many data collection elements such as `branchStorage`, `conflictStorage`, and `messageStorage` among others.

It can be described by the following characteristics:

- Is a manual cache which keeps objects in memory as long as consumers are using it.
- Uses key:value storage type .
- Provides mutex options for guarding shared variables, and preventing changing the object state by multiple goroutines at the same time.
- Takes care of  dynamic creation of different object types depending on the key, and the serialized data it receives through the utility `objectstorage.Factory`.
- Helps with the creation of multiple `ObjectStorage` instances from the same package, and  automatic configuration.

In order to create an object storage, we need to provide the underlying `kvstore.KVStore` structure backed by the database.

## Database

GoShimmer stores data in the form of an object storage system. The data is stored in one large repository with flat structure. Because of its categorization structure, it is a scalable solution that allows for fast data retrieval.

Additionally, GoShimmer leaves the possibility to store data only in memory that can be specified with the parameter `CfgDatabaseInMemory` value. The in-memory storage is purely based on a Go map, package `mapdb` from hive.go.

GoShimmer uses `RocksDB` for the persistent storage in a database. It is a fast key:value database, that performs well for both reads and writes simultaneously.  It was chosen because of its low memory consumption. 

Both solutions are implemented in the `database` package, along with prefix definitions that can be used during the creation of new object storage elements.

The database plugin is responsible for creating a `store` instance of the chosen database under the directory specified by the `CfgDatabaseDir` parameter. It will manage a proper closure of the database upon receiving a shutdown signal. During the start configuration, the database will be marked as unhealthy, and it will be marked as healthy on shutdown. After this, the garbage collector will run, and the database can be closed.

## ObjectStorage

Assume you need to store data for some newly created object `A`. Then, you need to define a new prefix for your package `A` in the `database` package, and prefixes for single storage objects. They will later be used during the `ObjectStorage` creation. A package prefix will be combined with a store specific prefix to create a specific realm.

```go
package example

type Storage struct {
	A                   *objectstorage.ObjectStorage
	...
	shutdownOnce        sync.Once
}
```

### ObjectStorage Factory

The most convenient way create multiple storage objects instances for one package is to use the factory function.

```go
osFactory := objectstorage.NewFactory(store, database.Prefix)
```

It needs two parameters:

- `store`: The key value `kvstore` instance
- `database.Prefix`: A prefix defined in the `database` package for your new `example` package. It will be responsible for the automatic configuration of the newly provided `kvstore` instance.

After defining the storage factory for the group of objects, you can use it to create an `*objectstorage.ObjectStorage` instance:

```go
AStorage = osFactory.New(objPrefix, FromObjectStorage)
AStorage = osFactory.New(objPrefix, FromObjectStorage, optionalOptions...)
```

For the function parameter you should provide:

- `objPrefix`: As mentioned before, we provide the object's specific prefix.
- `FromObjectStorage`: A function that allows the dynamic creation of different object types depending on the stored data.
- `optionalOptions`: An optional parameter provided in the form of an options array `[]objectstorage.Option`. All possible options should be defined in `objectstorage.Options`. If you do not specify them during creation, the default values will be used, such as enabled persistence, or setting cache time to 0.

### StorableObject

`StorableObject` is an interface that allows the dynamic creation of different object types depending on the stored data. To use the object storage factory, you will need to make sure that all you have implemented all the methods required by the interface. 

- `SetModified`: Marks the object as modified, which will be written to the disk (if you enabled persistence).
- `IsModified`: Returns true if the object is marked as modified.
- `Delete`: Marks the object to be deleted from the persistence layer.
- `IsDeleted`: Returns true if the object was marked as deleted.
- `Persist`: Enables or disables persistence for this object.
- `ShouldPersist`: Returns true if this object is going to be persisted.
- `Update`: Updates the object with the values of another object. **This method requires an explicit implementation**.
- `ObjectStorageKey`: Returns the key that is used to store the object in the database. **This method requires an explicit implementation**.
- `ObjectStorageValue`: Marshals the object data into a sequence of bytes that are used as the value part in the object storage. **This method requires an explicit implementation**.

:::info
You can find default implementation for most of these methods in the `objectstorage` library, except from `Update`, `ObjectStorageKey`, `ObjectStorageValue` which need to be provided.
:::

### StorableObjectFactory Function
The function `ObjectFromObjectStorage` from object storage provides functionality to restore objects from the `ObjectStorage`. By convention, the implementation of this function usually follows the schema:

`ObjectFromObjectStorage` uses `ObjectFromBytes`

```go
func ObjectFromObjectStorage(key []byte, data []byte) (result StorableObject, err error) {
    result, err := ObjectFromBytes(marshalutil.New(data))
    ...
    return
}
```

`ObjectFromBytes` unmarshals the object sequence of bytes with a help of the `marshalutil` library. The returned `consumedBytes` can be used for the testing purposes.
The created `marshalUtil` instance stores the stream of bytes, and keeps track of what has been already read (`readOffset`).

```go
func ObjectFromBytes(bytes []byte) (object *ObjectType, consumedBytes int, err error) {
    marshalUtil := marshalutil.New(bytes)
    if object, err = ObjectFromMarshalUtil(marshalUtil); err != nil {
    ...
    consumedBytes = marshalUtil.ReadOffset()
    return
}
```

The key logic that takes the marshaled object and transforms it into the object of specified type is implemented in `ObjectFromMarshalUtil` .

Because the data is stored in a sequence of bytes, it has no information about the form of an object and any data types it had before writing to the database.Therefore, you need to serialize any data into a stream of bytes in order to write it (marshaling), and deserialize the stream of bytes back into correct data structures when reading it (unmarshaling). 

The following is an example of unmarshaling of the `Approver` object.

```go
type Approver struct {
    approverType            ApproverType    //  8 bytes
    referencedMessageID     MessageID       // 32 bytes
    approverMessageID       MessageID       // 32 bytes
}
```

The order in which we read bytes has to reflect the order in which it was written down during marshaling. In the example, the order: `referencedMessageID`, `approverType`, `approverMessageID` is the same in both marshalling and unmarshalling.

```go
// Unmarshalling
func ApproverFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *Approver) {
    result = &Approver{}
    result.referencedMessageID = MessageIDFromMarshalUtil(marshalUtil)
    result.approverType = ApproverTypeFromMarshalUtil(marshalUtil)
    result.approverMessageID = MessageIDFromMarshalUtil(marshalUtil)
    return
}
// Marshalling
func (a *Approver) ObjectStorageApprover() []byte {
    return marshalutil.New().
    Write(a.referencedMessageID).
    Write(a.approverType).
    Write(a.approverMessageID).
    Bytes()
}
```

You can continue to decompose our object into smaller pieces with help of `MarshalUtil` struct, that keeps track of bytes, and a read offset.

You can use `marshalutil` build in methods on the appropriate parts of the byte stream with its length defined by the data type of the struct field. This way, you will be able to parse bytes to the correct Go data structure.

### ObjectStorage Methods

After defining the marshalling and unmarshalling mechanism for the `objectStorage` bytes conversion, you can start using it to store and read the particular parts of the project elements. 

- `Load`: Allows retrieving the corresponding object based on the provided id. For example, the method on the message `objectStorage`  is getting the cached object. 
- `Unwrap`: Converts a retrieved cached object to its own corresponding type.  In the code below it will return the message wrapped by the cached object.
- `Exists`: Checks if the object has been deleted. If so, it is released from memory with the `Release` method.
  
  ```go
  func (s *Storage) Message(messageID MessageID) *CachedMessage {
      return &CachedMessage{CachedObject: s.messageStorage.Load(messageID[:])}
  }
  
  cachedMessage := messagelayer.Tangle().Storage.Message(msgID)
  if !cachedMessage.Exists() {
      msgObject.Release()
      }
  message := cachedMessage.Unwrap()
  ``` 

- `Consume`: Unwraps the `CachedObject`, and passes a type-casted version to the consumer function. It will be useful when you want to apply a function on the cached object.  As soon as the object is consumed, and the callback is finished, the object is released.

  ```go
  cachedMessage.Consume(func(message *tangle.Message) {
              doSomething(message)
          })
  ```
- `ForEach`: Applies a `Consumer` function for every object residing within the cache, and the underlying persistence layer.  For example, this is how you can count the number of messages.
  
  ```go
  messageCount := 0
  messageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			messageCount++
        })
  }
  ```
  
- `Store`: Stores an object in the objectStorage. `StoreIfAbsent` is an extended version is this method that stores an object only if it was not stored before.  `StoreIfAbsent` returns boolean indication if the object was stored.
  `ComputeIfAbsent` works similarly but does not access the value log. 
  
    ```go
    cachedMessage := messageStorage.Store(newMessage)
    cachedMessage, stored := messageStorage.StoreIfAbsent(newMessage)
    cachedMessage := messageStorage.ComputeIfAbsent(newMessage, remappingFunction)
    ```
  
