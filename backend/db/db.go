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

func Open() (*mongo.Database, error) {
	uri := strings.TrimSpace(os.Getenv("MONGODB_URI"))
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
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

	clicksDateIdx := mongo.IndexModel{
		Keys: bson.D{{Key: "clicked_at", Value: 1}},
	}
	if _, err := database.Collection("clicks").Indexes().CreateOne(ctx, clicksDateIdx); err != nil {
		return fmt.Errorf("clicks clicked_at index: %w", err)
	}

	return nil
}
