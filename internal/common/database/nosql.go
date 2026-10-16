package database

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func NewNosqlDB(dbHost, dbPort, dbUser, dbPass string) (*mongo.Client, error) {
	uri := &url.URL{
		Scheme:   "mongodb",
		User:     url.UserPassword(dbUser, dbPass),
		Host:     fmt.Sprintf("%s:%s", dbHost, dbPort),
		Path:     "/",
		RawQuery: "authSource=admin",
	}

	opts := options.Client().ApplyURI(uri.String())

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		lastErr = client.Ping(ctx, nil)
		cancel()
		if lastErr == nil {
			return client, nil
		}
		time.Sleep(time.Second)
	}

	return nil, lastErr
}
