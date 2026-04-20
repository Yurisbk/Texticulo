package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MetricsHandler struct {
	DB *mongo.Database
}

type topLink struct {
	ShortCode   string `json:"short_code"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	Clicks      int64  `json:"clicks"`
}

type dayClicks struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

func (h *MetricsHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	totalLinks, _ := h.DB.Collection("links").CountDocuments(ctx, bson.M{})
	totalUsers, _ := h.DB.Collection("users").CountDocuments(ctx, bson.M{})

	// Sum all clicks via aggregation
	var totalClicksResult []struct {
		Total int64 `bson:"total"`
	}
	totalClicksPipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$clicks"}}},
		}}},
	}
	if cur, err := h.DB.Collection("links").Aggregate(ctx, totalClicksPipeline); err == nil {
		_ = cur.All(ctx, &totalClicksResult)
	}
	var totalClicks int64
	if len(totalClicksResult) > 0 {
		totalClicks = totalClicksResult[0].Total
	}

	// Clicks today
	today := time.Now().UTC().Truncate(24 * time.Hour)
	clicksToday, _ := h.DB.Collection("clicks").CountDocuments(ctx, bson.M{
		"clicked_at": bson.M{"$gte": today},
	})

	// Top 5 links by clicks
	topOpts := options.Find().
		SetSort(bson.D{{Key: "clicks", Value: -1}}).
		SetLimit(5)
	topCursor, err := h.DB.Collection("links").Find(ctx, bson.M{}, topOpts)
	var topLinks []topLink
	if err == nil {
		var docs []struct {
			ShortCode   string `bson:"short_code"`
			OriginalURL string `bson:"original_url"`
			Clicks      int64  `bson:"clicks"`
		}
		_ = topCursor.All(ctx, &docs)
		for _, d := range docs {
			if d.Clicks == 0 {
				continue
			}
			topLinks = append(topLinks, topLink{
				ShortCode:   d.ShortCode,
				ShortURL:    publicBase() + "/" + d.ShortCode,
				OriginalURL: d.OriginalURL,
				Clicks:      d.Clicks,
			})
		}
	}
	if topLinks == nil {
		topLinks = []topLink{}
	}

	// Clicks last 7 days
	sevenDaysAgo := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -6)
	clicksPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "clicked_at", Value: bson.D{{Key: "$gte", Value: sevenDaysAgo}}},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: "%Y-%m-%d"},
					{Key: "date", Value: "$clicked_at"},
				}},
			}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}
	var last7Days []dayClicks
	if daysCursor, err := h.DB.Collection("clicks").Aggregate(ctx, clicksPipeline); err == nil {
		var rawDays []struct {
			Date  string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		_ = daysCursor.All(ctx, &rawDays)
		for _, d := range rawDays {
			last7Days = append(last7Days, dayClicks{Date: d.Date, Count: d.Count})
		}
	}
	if last7Days == nil {
		last7Days = []dayClicks{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"total_links":        totalLinks,
		"total_clicks":       totalClicks,
		"total_users":        totalUsers,
		"clicks_today":       clicksToday,
		"top_links":          topLinks,
		"clicks_last_7_days": last7Days,
	})
}
