package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"password_hash" json:"-"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
}

type Link struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	ShortCode   string              `bson:"short_code" json:"short_code"`
	OriginalURL string              `bson:"original_url" json:"original_url"`
	UserID      *primitive.ObjectID `bson:"user_id,omitempty" json:"user_id,omitempty"`
	Clicks      int64               `bson:"clicks" json:"clicks"`
	CreatedAt   time.Time           `bson:"created_at" json:"created_at"`
}

type Click struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	LinkID    primitive.ObjectID `bson:"link_id" json:"link_id"`
	ClickedAt time.Time          `bson:"clicked_at" json:"clicked_at"`
	IPAddress string             `bson:"ip_address,omitempty" json:"ip_address,omitempty"`
	UserAgent string             `bson:"user_agent,omitempty" json:"user_agent,omitempty"`
}
