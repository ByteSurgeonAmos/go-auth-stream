package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)


type Post struct {
	ID primitive.ObjectID `bson:"_id" json:"id"`
	Content string `bson:"content" json:"content"`
	Images []string `bson:"images" json:"images"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	PostedAt time.Time `bson:"posted_at" json:"posted_at"`
	ScheduledAt time.Time `bson:"scheduled_at" json:"scheduled_at"`
	Status string `bson:"status" json:"status"`
	UserID primitive.ObjectID `bson:"user_id" json:"user_id"`
}
