package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)


type LoginInput struct {
	Email string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type SignupInput struct {
	UserName string `json:"username" binding:"required"`
	Email string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type SocialAccount struct {
    ID             primitive.ObjectID `bson:"_id,omitempty"`
    UserID         primitive.ObjectID `bson:"user_id"`
    Platform       string             `bson:"platform"`
    EncryptedToken string             `bson:"encrypted_token"`
    CreatedAt      time.Time          `bson:"created_at"`
    UpdatedAt      time.Time          `bson:"updated_at"`
}
