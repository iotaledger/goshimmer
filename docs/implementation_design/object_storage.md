# Object storage
In GoShimmer `ObjectStorage`  is used as a base data structure for many data collections elements such as `branchStorage`, `conflictStorage`, `messageStorage` and others.
It can be described by the following characteristics, it:
-  is a manual cache which keeps objects as long as consumers are using it.
- uses key-value storage type, defines its object factory, and mutex options for guarding shared variables and preventing changing object state by multiple gorutines at the same time.
- takes care of  dynamic creation of different object types depending on the key and the serialized data it recieves through the utility `objectstorage.Factory`.
- helps with multiple ObjectStorage instances creation from the same package and  automatic configuaration.

In order to create an object storage we need to provide underlying `kvstore.KVStore` structure backed by the database.



## Database
GoShimmer stores data in a form of an object storage system. The data is stored in one large repository with flat structure. It is a scalable solution that allows for fast data retrieval because of its categorization structure.

Additionally, GoShimmer leaves the possibility to store data only in memory that can be specified with the parameter `CfgDatabaseInMemory` value. In-memory storage is purely based on Go map, package `mapdb` from hive.go.
For the persistent storage in a database it uses package `badger` from hive.go. It is a simple and fast key-value database that performs well for both reads and writes simultaneously.

Both solutions are implemented in `database` package, along prefixes definitions that can be used during creation of new object storage elements.

Database plugin is responsible for creating `store` instance of the chosen database under the directory specified with `CfgDatabaseDir` parameter. It will manage proper closure of the database upon recieving shutdown signal. During the start configuration, database is marked as unhealthy, it will be marked as healthy on shutdown, then garbage collector is run and database can be closed.

## ObjectStorage


Assume we need to store data for some newly created object `A`. Then we need to define a new prefix for our package in the `database` package, and prefixes for single storage objects. They will be later used during objectstorage creation. A package prefix will be combined with a store specific prefix to create a specific realm.
```Go
package example

type Storage struct {
	A 						*objectstorage.ObjectStorage
	...
	shutdownOnce			sync.Once
}
```
### ObjectStorage factory
To easily create multiple storage objects instances for one package, the most convenient way is to use the factory function.
```Go
osFactory := objectstorage.NewFactory(store, database.Prefix)
```
It needs two parameters:
- `store` - the key value `kvstore` instance
- `database.Prefix` - prefix defined in the `database` package for our new `example` package. It will be responsible for automatic configuration of newly provided KVStore object.


After defining storage factory for the group of object we can use it to create `*objectstorage.ObjectStorage` instance:
```Go
AStorage = osFactory.New(objPrefix, FromObjectStorage)
AStorage = osFactory.New(objPrefix, FromObjectStorage, optionalOptions...)
```
For the function parameter we should provide:
- `objPrefix` - mentioned before, we provide the object specyfic prefix.
- `FromObjectStorage` function, a `StorableObject` factory that receives a key and the serialized data of the object and returns an empty `StorableObject` with just key set.
- `optionalOptions` -  an optional parameter provided in a form of options array `[]objectstorage.Option`. All possible options are defined in `objectstorage.Options`. If we do not specify them during creation, the default values will be used, such as enabling persistance or setting cache time to 0.

### StorableObject
`StorableObject` is an interface that allow the dynamic creation of different object types depending on the stored data. We need to make sure that all methods required by the interface are implemented to use object storage factory.

- `SetModified` - marks the object as modified, which will be written to the disk (if persistence is enabled).
- `IsModified` - returns true if the object is marked as modified
- `Delete` - marks the object to be deleted from the persistence layer
- `IsDeleted` - returns true if the object was marked as deleted
- `Persist` - enables or disables persistence for this object
- `ShouldPersist` - returns "true" if this object is going to be persisted
- `Update` - updates the object with the values of another object - needs to be implemented
- `ObjectStorageKey` - returns the key that is used to store the object in the database - needs to be implemented
- `ObjectStorageValue` - marshals the object data into a sequence of bytes that are used as the value part in the object storage - needs to be implemented

Most of them has its default implementation in `objectstorage` library except from `Update`, `ObjectStorageKey`, `ObjectStorageValue` which need to be provided.

### StorableObjectFactory function
The function `FromObjectStorage` from object storage provides functionality to restore objects from the objectstorage. By convention the implementation of this function usually follows the schema:
`ObjectFromObjectStorage` uses `ObjectFromBytes`
```Go
func FromObjectStorage(key []byte, data []byte) (result StorableObject, err error) {
	result, err := ObjectFromBytes(marshalutil.New(data))
	if err != nil {
		return
	}
	...
	return
}
```

`ObjectFromBytes` unmarshals the object sequence of bytes with help of `marshalutil` library.

The key logic needs to be implemented in `ObjectFromMarshalUtil` that takes the marshaled object and returns an object with an appropriate object type.
