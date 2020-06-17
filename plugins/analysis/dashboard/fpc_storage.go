package dashboard

import (
	"context"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/plugins/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// FPCRecord defines the FPC record to be stored into a mongoDB
type FPCRecord struct {
	ConflictID string  `json:"conflictid" bson:"conflictid"`
	NodeID     string  `json:"nodeid" bson:"nodeid"`
	Rounds     int     `json:"rounds" bson:"rounds"`
	Opinions   []int32 `json:"opinions" bson:"opinions"`
	Status     int32   `json:"status" bson:"status"`
}

var (
	db              *mongo.Database
	ctxDisconnectDB context.Context
	clientDB        *mongo.Client
	dbOnce          sync.Once
)

func shutdownMongoDB() {
	clientDB.Disconnect(ctxDisconnectDB)
}

func mongoDB() *mongo.Database {
	dbOnce.Do(func() {
		username := config.Node.GetString(CfgMongoDBUsername)
		password := config.Node.GetString(CfgMongoDBPassword)
		bindAddr := config.Node.GetString(CfgMongoDBBindAddress)
		client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://" + username + ":" + password + "@" + bindAddr))
		if err != nil {
			log.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = client.Connect(ctx)
		ctxDisconnectDB = ctx
		defer cancel()
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = client.Ping(ctx, readpref.Primary())
		if err != nil {
			log.Fatal(err)
		}
		db = client.Database("analysis")
	})
	return db
}

func storeFPCRecords(records []FPCRecord, db *mongo.Database) error {
	data := make([]interface{}, len(records))
	for i := range records {
		data[i] = records[i]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.Collection("FPC").InsertMany(ctx, data)
	return err
}
