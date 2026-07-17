package database

import (
	"context"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
)

// ConnectMongoDB establishes a connection to MongoDB.
// It returns the client, the specific database object, and a cancel function to disconnect when needed.
func ConnectMongoDB(ctx context.Context, uri string, dbName string) (*mongo.Client, *mongo.Database, error) {
	clientOptions := options.Client().ApplyURI(uri)

	if os.Getenv("OTEL_ENABLED") == "true" {
		clientOptions.SetMonitor(otelmongo.NewMonitor())
	}

	// Set connection timeout
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(connectCtx, clientOptions)
	if err != nil {
		return nil, nil, err
	}

	// Ping database to verify connection
	pingCtx, pingCancel := context.WithTimeout(ctx, 2*time.Second)
	defer pingCancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, nil, err
	}

	db := client.Database(dbName)
	return client, db, nil
}
