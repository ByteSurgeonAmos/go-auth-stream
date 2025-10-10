package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Post struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Content     string             `bson:"content" json:"content"`
	Images      []string           `bson:"images" json:"images"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	PublishedAt time.Time          `bson:"published_at,omitempty" json:"published_at,omitempty"`
	ScheduledAt time.Time          `bson:"scheduled_at,omitempty" json:"scheduled_at,omitempty"`
	Status      string             `bson:"status" json:"status"` // draft, scheduled, published, failed
	UserID      primitive.ObjectID `bson:"user_id" json:"user_id"`
	CompanyName string             `bson:"company_name,omitempty" json:"company_name,omitempty"`
	Platforms   []string           `bson:"platforms,omitempty" json:"platforms,omitempty"`
	ImageURL    string             `bson:"image_url,omitempty" json:"image_url,omitempty"`
	ErrorMsg    string             `bson:"error_msg,omitempty" json:"error_msg,omitempty"`
	RetryCount  int                `bson:"retry_count" json:"retry_count"`
}
