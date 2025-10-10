package types

import (
	"time"

	"github.com/ByteSurgeonAmos/go-auth-stream/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type SignupInput struct {
	UserName string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type TwoFactorVerifyInput struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

type VerifyAccountInput struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

type RefreshTokenInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type UpdateSocialMediaInput struct {
	CompanyName string        `json:"company_name" binding:"required"`
	Social      models.Social `json:"social" binding:"required"`
}

type CreatePostInput struct {
	Content     string     `json:"content" binding:"required"`
	Images      []string   `json:"images"`
	ScheduledAt *time.Time `json:"scheduled_at"`
	CompanyName string     `json:"company_name"`
	Platforms   []string   `json:"platforms"`
	ImageURL    string     `json:"image_url"`
}

type CreateSubscriptionInput struct {
	PlanID string `json:"plan_id" binding:"required"`
}

type SocialAccount struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	UserID         primitive.ObjectID `bson:"user_id"`
	Platform       string             `bson:"platform"`
	EncryptedToken string             `bson:"encrypted_token"`
	CreatedAt      time.Time          `bson:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at"`
}
