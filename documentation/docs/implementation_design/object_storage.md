---
description: ObjectStorage is used as a base data structure for many data collection elements such as `branchStorage`, `conflictStorage`, `messageStorage` amongst others.
image: /img/logo/goshimmer_light.png
keywords:
- storage
- dynamic creation
- database
- parameters
- object types
- stream of bytes
- cached
---

# Object Storage

In GoShimmer `ObjectStorage`  is used as a base data structure for many data collection elements such as `branchStorage`, `conflictStorage`, `messageStorage` and others.
It can be described by the following characteristics, it:
- is a manual cache which keeps objects in memory as long as consumers are using it
- uses key-value storage type 
- provides mutex options for guarding shared variables and preventing changing the object state by multiple goroutines at the same time
- takes care of  dynamic creation of different object types depending on the key, and the serialized data it receives through the utility `objectstorage.Factory`
- helps with the creation of multiple `ObjectStorage` instances from the same package and  automatic configuration.

In order to create an object storage we need to provide the underlying `kvstore.KVStore` structure backed by the database.

## Database

GoShimmer stores data in the form of an object storage system. The data is stored in one large repository with flat structure. It is a scalable solution that allows for fast data retrieval because of its categorization structure.

Additionally, GoShimmer leaves the possibility to store data only in memory that can be specified with the parameter `CfgDatabaseInMemory` value. In-memory storage is purely based on a Go map, package `mapdb` from hive.go.
For the persistent storage in a database it uses `RocksDB`. It is a fast key-value database that performs well for both reads and writes simultaneously that was chosen due to its low memory consumption. 

Both solutions are implemented in the `database` package, along with prefix definitions that can be used during the creation of new object storage elements.

The database plugin is responsible for creating a `store` instance of the chosen database under the directory specified with `CfgDatabaseDir` parameter. It will manage a proper closure of the database upon receiving a shutdown signal. During the start configuration, the database is marked as unhealthy, and it will be marked as healthy on shutdown. Then the garbage collector is run and the database can be closed.

## ObjectStorage


Assume we need to store data for some newly created object `A`. Then we need to define a new prefix for our package in the `database` package, and prefixes for single storage objects. They will be later used during `ObjectStorage` creation. A package prefix will be combined with a store specific prefix to create a specific realm.
```Go
package example

type Storage struct {
	A                   *objectstorage.ObjectStorage
	...
	shutdownOnce        sync.Once
}
```

### ObjectStorage Factory

To easily create multiple storage objects instances for one package, the most convenient way is to use the factory function.
```Go
osFactory := objectstorage.NewFactory(store, database.Prefix)
```
It needs two parameters:
- `store` - the key value `kvstore` instance
- `database.Prefix` - a prefix defined in the `database` package for our new `example` package. It will be responsible for automatic configuration of the newly provided `kvstore` instance.


After defining the storage factory for the group of objects, we can use it to create an `*objectstorage.ObjectStorage` instance:
```Go
AStorage = osFactory.New(objPrefix, FromObjectStorage)
AStorage = osFactory.New(objPrefix, FromObjectStorage, optionalOptions...)
```
For the function parameter we should provide:
- `objPrefix` - mentioned before, we provide the object specific prefix.
- `FromObjectStorage` - a function that allows the dynamic creation of different object types depending on the stored data.
- `optionalOptions` -  an optional parameter provided in the form of options array `[]objectstorage.Option`. All possible options are defined in `objectstorage.Options`. If we do not specify them during creation, the default values will be used, such as enabled persistence or setting cache time to 0.

### StorableObject

`StorableObject` is an interface that allows the dynamic creation of different object types depending on the stored data. We need to make sure that all methods required by the interface are implemented to use the object storage factory.

- `SetModified` - marks the object as modified, which will be written to the disk (if persistence is enabled).
- `IsModified` - returns true if the object is marked as modified
- `Delete` - marks the object to be deleted from the persistence layer
- `IsDeleted` - returns true if the object was marked as deleted
- `Persist` - enables or disables persistence for this object
- `ShouldPersist` - returns true if this object is going to be persisted
- `Update` - updates the object with the values of another object - requires an explicit implementation
- `ObjectStorageKey` - returns the key that is used to store the object in the database - requires an explicit implementation
- `ObjectStorageValue` - marshals the object data into a sequence of bytes that are used as the value part in the object storage - requires an explicit implementation

Most of these have their default implementation in `objectstorage` library, except from `Update`, `ObjectStorageKey`, `ObjectStorageValue` which need to be provided.

### StorableObjectFactory Function

The function `ObjectFromObjectStorage` from object storage provides functionality to restore objects from the `ObjectStorage`. By convention the implementation of this function usually follows the schema:
`ObjectFromObjectStorage` uses `ObjectFromBytes`
```Go
func ObjectFromObjectStorage(key []byte, data []byte) (result StorableObject, err error) {
    result, err := ObjectFromBytes(marshalutil.New(data))
    ...
    return
}
```

`ObjectFromBytes` unmarshals the object sequence of bytes with a help of `marshalutil` library. The returned `consumedBytes` can be used for the testing purposes.
The created `marshalUtil` instance stores the stream of bytes and keeps track of what has been already read (`readOffset`).
```Go
func ObjectFromBytes(bytes []byte) (object *ObjectType, consumedBytes int, err error) {
    marshalUtil := marshalutil.New(bytes)
    if object, err = ObjectFromMarshalUtil(marshalUtil); err != nil {
    ...
    consumedBytes = marshalUtil.ReadOffset()
    return
}
```
The key logic is implemented in `ObjectFromMarshalUtil` that takes the marshaled object and transforms it into the object of specified type.
Because the data is stored in a sequence of bytes, it has no information about the form of an object and any data types it had before writing to the database.
Thus, we need to serialize any data into a stream of bytes in order to write it (marshaling), and deserialize the stream of bytes back into correct data structures when reading it (unmarshaling). 
Let's consider as an example, unmarshaling of the `Approver` object.
```Go
type Approver struct {
    approverType            ApproverType    //  8 bytes
    referencedMessageID     MessageID       // 32 bytes
    approverMessageID       MessageID       // 32 bytes
}
```

The order in which we read bytes has to reflect the order in which it was written down during marshaling. As in the example, the order: `referencedMessageID`, `approverType`, `approverMessageID` is the same in both marshalling and unmarshalling.

```Go
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

We continue to decompose our object into smaller pieces with help of `MarshalUtil` struct that keeps track of bytes, and a read offset.
Then we use `marshalutil` build in methods on the appropriate parts of the byte stream with its length defined by the data
type of the struct field. This way, we are able to parse bytes to the correct Go data structure.

### ObjectStorage Methods

After defining marshalling and unmarshalling mechanism for`objectStorage` bytes conversion, 
we can start using it for its sole purpose, to actually store and read the particular parts of the project elements. 

 - `Load` allows retrieving the corresponding object based on the provided id. For example, the method on the message `objectStorage`  
  is getting the cached object. 
- To convert an object retrieved in the form of a cache to its own corresponding type, we can use `Unwrap`.
 In the code below it will return the message wrapped by the cached object.
- `Exists` - checks weather the object has been deleted. If so it is released from memory with the `Release` method.
    ```Go
    func (s *Storage) Message(messageID MessageID) *CachedMessage {
        return &CachedMessage{CachedObject: s.messageStorage.Load(messageID[:])}
    }
    
    cachedMessage := messagelayer.Tangle().Storage.Message(msgID)
    if !cachedMessage.Exists() {
        msgObject.Release()
        }
    message := cachedMessage.Unwrap()
    ``` 
- `Consume` will be useful when we want to apply a function on the cached object. `Consume` unwraps the `CachedObject` and passes a type-casted version to the consumer function.
  Right after the object is consumed and when the callback is finished, the object is released.

    ```Go
    cachedMessage.Consume(func(message *tangle.Message) {
                doSomething(message)
            })
    ```
- `ForEach` - allows to apply a `Consumer` function for every object residing within the cache and the underlying persistence layer.
  For example, this is how we can count the number of messages.
  ```Go
  messageCount := 0
  messageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedObject.Consume(func(object objectstorage.StorableObject) {
			messageCount++
        })
  }
  ```
- `Store` - storing an object in the objectStorage. An extended version is method `StoreIfAbsent` 
  that stores an object only if it was not stored before and returns boolean indication if the object was stored. 
  `ComputeIfAbsent` works similarly but does not access the value log. 
    ```Go
    cachedMessage := messageStorage.Store(newMessage)
    cachedMessage, stored := messageStorage.StoreIfAbsent(newMessage)
    cachedMessage := messageStorage.ComputeIfAbsent(newMessage, remappingFunction)
    ```
  
