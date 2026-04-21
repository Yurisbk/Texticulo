package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func sanitizeMongoURI(uri string) string {
	uri = strings.TrimSpace(uri)
	uri = strings.Trim(uri, `"'`)
	return uri
}

func Open() (*mongo.Database, error) {
	uri := sanitizeMongoURI(os.Getenv("MONGODB_URI"))
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := options.Client().ApplyURI(uri).
		SetServerSelectionTimeout(30 * time.Second).
		SetConnectTimeout(20 * time.Second)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	dbName := strings.TrimSpace(os.Getenv("MONGODB_DB"))
	if dbName == "" {
		dbName = "texticulo"
	}
	return client.Database(dbName), nil
}

func Migrate(database *mongo.Database) error {
	ctx := context.Background()

	usersIdx := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	if _, err := database.Collection("users").Indexes().CreateOne(ctx, usersIdx); err != nil {
		return fmt.Errorf("users email index: %w", err)
	}

	linksCodeIdx := mongo.IndexModel{
		Keys:    bson.D{{Key: "short_code", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	if _, err := database.Collection("links").Indexes().CreateOne(ctx, linksCodeIdx); err != nil {
		return fmt.Errorf("links short_code index: %w", err)
	}

	linksUserIdx := mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}},
	}
	if _, err := database.Collection("links").Indexes().CreateOne(ctx, linksUserIdx); err != nil {
		return fmt.Errorf("links user_id index: %w", err)
	}

	clicksIdx := mongo.IndexModel{
		Keys: bson.D{{Key: "link_id", Value: 1}},
	}
	if _, err := database.Collection("clicks").Indexes().CreateOne(ctx, clicksIdx); err != nil {
		return fmt.Errorf("clicks link_id index: %w", err)
	}

	// TTL index: clicks expire after 5 hours to save space.
	// Drop the plain clicked_at index first (if it exists) so we can replace it with a TTL version.
	_, _ = database.Collection("clicks").Indexes().DropOne(ctx, "clicked_at_1")
	clicksTTLIdx := mongo.IndexModel{
		Keys:    bson.D{{Key: "clicked_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(5 * 3600),
	}
	if _, err := database.Collection("clicks").Indexes().CreateOne(ctx, clicksTTLIdx); err != nil {
		return fmt.Errorf("clicks TTL index: %w", err)
	}

	// TTL index: anonymous links (no user_id) expire after 48 hours.
	anonLinksTTLIdx := mongo.IndexModel{
		Keys: bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index().
			SetExpireAfterSeconds(48 * 3600).
			SetPartialFilterExpression(bson.D{{Key: "user_id", Value: bson.D{{Key: "$exists", Value: false}}}}),
	}
	if _, err := database.Collection("links").Indexes().CreateOne(ctx, anonLinksTTLIdx); err != nil {
		return fmt.Errorf("anon links TTL index: %w", err)
	}

	return nil
}
