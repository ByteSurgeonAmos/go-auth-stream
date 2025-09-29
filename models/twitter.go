package models

import (
	"context"
	"fmt"
	"time"

	"github.com/ByteSurgeonAmos/go-auth-stream/internal/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/oauth2"
)

type TokenDocument struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	UserID       string             `bson:"user_id"`
	Platform     string             `bson:"platform"`
	AccessToken  string             `bson:"access_token"`
	RefreshToken string             `bson:"refresh_token"`
	TokenType    string             `bson:"token_type"`
	Expiry       time.Time          `bson:"expiry"`
	CreatedAt    time.Time          `bson:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at"`
}

const tokensCollection = "tokens"

var DB = db.DB

func SaveToken(ctx context.Context, userID, platform string, token *oauth2.Token) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	collection := DB.Collection(tokensCollection)

	tokenDoc := TokenDocument{
		UserID:       userID,
		Platform:     platform,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
		UpdatedAt:    time.Now(),
	}

	filter := bson.M{
		"user_id":  userID,
		"platform": platform,
	}

	update := bson.M{
		"$set": tokenDoc,
		"$setOnInsert": bson.M{
			"created_at": time.Now(),
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	return nil
}

func GetToken(ctx context.Context, userID, platform string) (*oauth2.Token, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	collection := DB.Collection(tokensCollection)

	filter := bson.M{
		"user_id":  userID,
		"platform": platform,
	}

	var tokenDoc TokenDocument
	err := collection.FindOne(ctx, filter).Decode(&tokenDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("token not found for user %s on platform %s", userID, platform)
		}
		return nil, fmt.Errorf("failed to retrieve token: %w", err)
	}

	token := &oauth2.Token{
		AccessToken:  tokenDoc.AccessToken,
		RefreshToken: tokenDoc.RefreshToken,
		TokenType:    tokenDoc.TokenType,
		Expiry:       tokenDoc.Expiry,
	}

	return token, nil
}

func DeleteToken(ctx context.Context, userID, platform string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	collection := DB.Collection(tokensCollection)

	filter := bson.M{
		"user_id":  userID,
		"platform": platform,
	}

	result, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("token not found for user %s on platform %s", userID, platform)
	}

	return nil
}

func IsTokenValid(ctx context.Context, userID, platform string) (bool, error) {
	token, err := GetToken(ctx, userID, platform)
	if err != nil {
		return false, err
	}

	return token.Expiry.After(time.Now().Add(5 * time.Minute)), nil
}

func GetAllUserTokens(ctx context.Context, userID string) ([]TokenDocument, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	collection := DB.Collection(tokensCollection)

	filter := bson.M{"user_id": userID}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tokens: %w", err)
	}
	defer cursor.Close(ctx)

	var tokens []TokenDocument
	if err := cursor.All(ctx, &tokens); err != nil {
		return nil, fmt.Errorf("failed to decode tokens: %w", err)
	}

	return tokens, nil
}

func UpdateTokenExpiry(ctx context.Context, userID, platform string, expiry time.Time) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	collection := DB.Collection(tokensCollection)

	filter := bson.M{
		"user_id":  userID,
		"platform": platform,
	}

	update := bson.M{
		"$set": bson.M{
			"expiry":     expiry,
			"updated_at": time.Now(),
		},
	}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update token expiry: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("token not found for user %s on platform %s", userID, platform)
	}

	return nil
}

func CleanupExpiredTokens(ctx context.Context) (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	collection := DB.Collection(tokensCollection)

	filter := bson.M{
		"expiry": bson.M{"$lt": time.Now()},
	}

	result, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired tokens: %w", err)
	}

	return result.DeletedCount, nil
}

func CreateTokenIndexes(ctx context.Context) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	collection := DB.Collection(tokensCollection)

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "platform", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "expiry", Value: 1},
			},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
			},
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}