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
}

const (
	// DefaultMongoDBOpTimeout defines the default MongoDB operation timeout.
	DefaultMongoDBOpTimeout = 5 * time.Second
)

var (
	db     *mongo.Database
	dbOnce sync.Once
)

func mongoDB() *mongo.Database {
	dbOnce.Do(func() {
		username := config.Node.GetString(CfgMongoDBUsername)
		password := config.Node.GetString(CfgMongoDBPassword)
		hostAddr := config.Node.GetString(CfgMongoDBHostAddress)

		client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://" + username + ":" + password + "@" + hostAddr))
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := operationTimeout(DefaultMongoDBOpTimeout)
		defer cancel()
		if err := client.Connect(ctx); err != nil {
			log.Fatal(err)
		}

		ctx, cancel = operationTimeout(DefaultMongoDBOpTimeout)
		defer cancel()
		if err := client.Ping(ctx, readpref.Primary()); err != nil {
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
	ctx, cancel := operationTimeout(DefaultMongoDBOpTimeout)
	defer cancel()
	_, err := db.Collection("FPC").InsertMany(ctx, data)
	return err
}

func operationTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
