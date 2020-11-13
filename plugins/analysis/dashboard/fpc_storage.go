package dashboard

import (
	"context"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// FPCRecord defines the FPC record to be stored into a mongoDB.
type FPCRecord struct {
	// ConflictID defines the ID of the conflict.
	ConflictID string `json:"conflictid" bson:"conflictid"`
	// NodeID defines the ID of the node.
	NodeID string `json:"nodeid" bson:"nodeid"`
	// Rounds defines number of rounds performed to finalize the conflict.
	Rounds int `json:"rounds" bson:"rounds"`
	// Opinions contains the opinion of each round.
	Opinions []int32 `json:"opinions" bson:"opinions"`
	// Outcome defines final opinion of the conflict.
	Outcome int32 `json:"outcome" bson:"outcome"`
	// Time defines the time when the conflict has been finalized.
	Time primitive.DateTime `json:"datetime" bson:"datetime"`
}

// FPCRecords defines a slice of FPCRecord
type FPCRecords = []FPCRecord

const (
	// defaultMongoDBOpTimeout defines the default MongoDB operation timeout.
	defaultMongoDBOpTimeout = 5 * time.Second
)

var (
	clientDB *mongo.Client
	//db       *mongo.Database
	dbOnce sync.Once
	// read locked by pingers and write locked by the routine trying to reconnect.
	mongoReconnectLock sync.RWMutex
)

func mongoDB() *mongo.Database {
	dbOnce.Do(func() {
		client, err := newMongoDB()
		if err != nil {
			log.Fatal(err)
		}
		clientDB = client
	})
	return clientDB.Database("analysis")
}

func newMongoDB() (*mongo.Client, error) {
	username := config.Node().String(CfgMongoDBUsername)
	password := config.Node().String(CfgMongoDBPassword)
	hostAddr := config.Node().String(CfgMongoDBHostAddress)

	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://" + username + ":" + password + "@" + hostAddr))
	if err != nil {
		log.Fatalf("MongoDB NewClient failed: %s", err)
		return nil, err
	}

	if err := connectMongoDB(client); err != nil {
		return nil, err
	}

	if err := pingMongoDB(client); err != nil {
		return nil, err
	}

	return client, nil
}

// pings the MongoDB and attempts to reconnect to it in case the connection was lost.
func pingOrReconnectMongoDB() error {
	mongoReconnectLock.RLock()
	ctx, cancel := operationTimeout(defaultMongoDBOpTimeout)
	defer cancel()
	err := clientDB.Ping(ctx, readpref.Primary())
	if err == nil {
		mongoReconnectLock.RUnlock()
		return nil
	}
	log.Warnf("MongoDB ping failed: %s", err)
	mongoReconnectLock.RUnlock()

	mongoReconnectLock.Lock()
	defer mongoReconnectLock.Unlock()

	// check whether ping still doesn't work
	ctx, cancel = operationTimeout(defaultMongoDBOpTimeout)
	defer cancel()
	if err := clientDB.Ping(ctx, readpref.Primary()); err == nil {
		return nil
	}

	// reconnect
	ctx, cancel = operationTimeout(defaultMongoDBOpTimeout)
	defer cancel()
	if err := clientDB.Connect(ctx); err != nil {
		log.Warnf("MongoDB connection failed: %s", err)
		return err
	}
	return nil
}

func connectMongoDB(client *mongo.Client) error {
	ctx, cancel := operationTimeout(defaultMongoDBOpTimeout)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		log.Warnf("MongoDB connection failed: %s", err)
		return err
	}
	return nil
}

func pingMongoDB(client *mongo.Client) error {
	ctx, cancel := operationTimeout(defaultMongoDBOpTimeout)
	defer cancel()
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Warnf("MongoDB ping failed: %s", err)
		return err
	}
	return nil
}

func storeFPCRecords(records FPCRecords, db *mongo.Database) error {
	data := make([]interface{}, len(records))
	for i := range records {
		data[i] = records[i]
	}
	ctx, cancel := operationTimeout(defaultMongoDBOpTimeout)
	defer cancel()
	_, err := db.Collection("FPC").InsertMany(ctx, data)
	return err
}

func operationTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
