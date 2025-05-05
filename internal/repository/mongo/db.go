package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Default connection timeout
const defaultTimeout = 10 * time.Second

// ConnectDB establishes a connection to MongoDB using the provided URI.
// It returns the mongo.Client which can be used to access databases and collections.
func ConnectDB(uri string) (*mongo.Client, error) {
	// Set context with timeout for the connection attempt
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel() // Ensure cancellation happens even if connection errors out

	// Set client options from the URI
	clientOptions := options.Client().ApplyURI(uri)

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the primary node to verify the connection.
	// Use a separate context for the ping, as the initial connection might have succeeded
	// but the server might be unresponsive.
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second) // Shorter timeout for ping
	defer pingCancel()

	err = client.Ping(pingCtx, readpref.Primary())
	if err != nil {
		// If ping fails, disconnect the client before returning the error
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		_ = client.Disconnect(disconnectCtx) // Log or ignore disconnect error here
		return nil, err
	}

	// Connection successful
	return client, nil
}

// DisconnectDB gracefully disconnects the MongoDB client.
func DisconnectDB(client *mongo.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	return client.Disconnect(ctx)
}

// Note: You might want to manage the client lifecycle (connect/disconnect)
// more centrally in your main.go or using a dependency injection framework.
// Passing the *mongo.Database instance (obtained via client.Database("dbName"))
// to repositories is often preferred over passing the *mongo.Client.
