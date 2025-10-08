package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ByteSurgeonAmos/go-auth-stream/connectors"
	"github.com/ByteSurgeonAmos/go-auth-stream/internal/db"
	"github.com/ByteSurgeonAmos/go-auth-stream/internal/middleware"
	"github.com/ByteSurgeonAmos/go-auth-stream/models"
	"github.com/ByteSurgeonAmos/go-auth-stream/types"
	"github.com/ByteSurgeonAmos/go-auth-stream/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
)

var postsCollection *mongo.Collection

func InitPostHandler() {
	postsCollection = db.DB.Collection("posts")
}

func CreatePost(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	var input types.CreatePostInput
	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	status := "draft"
	var scheduledAt time.Time
	if input.ScheduledAt != nil {
		status = "scheduled"
		scheduledAt = *input.ScheduledAt
	}

	post := models.Post{
		ID:          primitive.NewObjectID(),
		Content:     input.Content,
		Images:      input.Images,
		CreatedAt:   time.Now(),
		ScheduledAt: scheduledAt,
		Status:      status,
		UserID:      userObjID,
	}

	result, err := postsCollection.InsertOne(ctx, post)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating post"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Post created successfully",
		"post_id": result.InsertedID,
		"post":    post,
	})
}

func GetAllPosts(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	filter := bson.M{"user_id": objID}
	cursor, err := postsCollection.Find(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching posts"})
		return
	}
	defer cursor.Close(ctx)

	var posts []models.Post
	err = cursor.All(ctx, &posts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding posts"})
		return
	}

	if posts == nil {
		posts = []models.Post{}
	}

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
		"count": len(posts),
	})
}

func PublishPost(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	var input struct {
		PostID      string   `json:"post_id" binding:"required"`
		CompanyName string   `json:"company_name" binding:"required"`
		Platforms   []string `json:"platforms" binding:"required"`
		ImageURL    string   `json:"image_url,omitempty"`
	}

	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := utils.TimeoutWindow(30)
	defer cancel()

	postObjID, err := primitive.ObjectIDFromHex(input.PostID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	var post models.Post
	err = postsCollection.FindOne(ctx, bson.M{"_id": postObjID}).Decode(&post)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	userObjID, _ := primitive.ObjectIDFromHex(userID)
	var user models.User
	err = db.DB.Collection("users").FindOne(ctx, bson.M{"_id": userObjID}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching user"})
		return
	}

	var company *models.Company
	for _, comp := range user.Companies {
		if comp.CompanyName == input.CompanyName {
			company = &comp
			break
		}
	}

	if company == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Company not found"})
		return
	}

	results := make(map[string]interface{})

	for _, platformName := range input.Platforms {
		platform := models.Platform(platformName)

		var social *models.Social
		for _, s := range company.Socials {
			if s.Platform == platform {
				social = &s
				break
			}
		}

		if social == nil {
			results[platformName] = gin.H{
				"success": false,
				"error":   "Platform not connected",
			}
			continue
		}

		switch platform {
		case models.PlatformX:
			postID, err := publishToTwitter(ctx, userObjID, input.CompanyName, social, post.Content)
			if err != nil {
				results[platformName] = gin.H{
					"success": false,
					"error":   err.Error(),
				}
			} else {
				results[platformName] = gin.H{
					"success": true,
					"post_id": postID,
				}
			}

		case models.PlatformFacebook:
			postID, err := publishToFacebook(ctx, userObjID, input.CompanyName, social, post.Content)
			if err != nil {
				results[platformName] = gin.H{
					"success": false,
					"error":   err.Error(),
				}
			} else {
				results[platformName] = gin.H{
					"success": true,
					"post_id": postID,
				}
			}

		case models.PlatformInstagram:
			if input.ImageURL == "" {
				results[platformName] = gin.H{
					"success": false,
					"error":   "Image URL required for Instagram",
				}
				continue
			}
			postID, err := publishToInstagram(ctx, userObjID, input.CompanyName, social, post.Content, input.ImageURL)
			if err != nil {
				results[platformName] = gin.H{
					"success": false,
					"error":   err.Error(),
				}
			} else {
				results[platformName] = gin.H{
					"success": true,
					"post_id": postID,
				}
			}
		}
	}

	_, err = postsCollection.UpdateOne(ctx, bson.M{"_id": postObjID}, bson.M{
		"$set": bson.M{
			"status":       "published",
			"published_at": time.Now(),
		},
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Post published",
		"results": results,
	})
}

func publishToTwitter(ctx context.Context, userID primitive.ObjectID, companyName string, social *models.Social, content string) (string, error) {
	token := &oauth2.Token{
		AccessToken:  social.AccessToken,
		RefreshToken: social.RefreshToken,
		Expiry:       social.ExpiresAt,
	}

	postID, newToken, err := connectors.PostTweetWithRetry(ctx, token, content)
	if err != nil {
		return "", err
	}

	// Update token if refreshed
	if newToken.AccessToken != token.AccessToken {
		if err := UpdateSocialToken(userID, companyName, social.Platform, newToken.AccessToken, newToken.RefreshToken, newToken.Expiry); err != nil {
			log.Printf("Warning: Failed to update Twitter token in database: %v", err)
		}
	}

	return postID, nil
}

func publishToFacebook(ctx context.Context, userID primitive.ObjectID, companyName string, social *models.Social, content string) (string, error) {
	token := &oauth2.Token{
		AccessToken:  social.AccessToken,
		RefreshToken: social.RefreshToken,
		Expiry:       social.ExpiresAt,
	}

	// Use the username field to store page ID
	postID, newToken, err := connectors.PostToFacebookWithRetry(ctx, token, social.Username, content)
	if err != nil {
		return "", err
	}

	// Update token if refreshed
	if newToken.AccessToken != token.AccessToken {
		if err := UpdateSocialToken(userID, companyName, social.Platform, newToken.AccessToken, newToken.RefreshToken, newToken.Expiry); err != nil {
			log.Printf("Warning: Failed to update Facebook token in database: %v", err)
		}
	}

	return postID, nil
}

func publishToInstagram(ctx context.Context, userID primitive.ObjectID, companyName string, social *models.Social, caption, imageURL string) (string, error) {
	token := &oauth2.Token{
		AccessToken: social.AccessToken,
		Expiry:      social.ExpiresAt,
	}

	// Use the username field to store Instagram account ID
	postID, newToken, err := connectors.PostToInstagramWithRetry(ctx, token, social.Username, caption, imageURL)
	if err != nil {
		return "", err
	}

	// Update token if refreshed
	if newToken.AccessToken != token.AccessToken {
		if err := UpdateSocialToken(userID, companyName, social.Platform, newToken.AccessToken, "", newToken.Expiry); err != nil {
			log.Printf("Warning: Failed to update Instagram token in database: %v", err)
		}
	}

	return postID, nil
}

// UpdateSocialToken updates OAuth tokens for a specific platform in the user's company
func UpdateSocialToken(userID primitive.ObjectID, companyName string, platform models.Platform, accessToken, refreshToken string, expiresAt time.Time) error {
	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	usersCollection := db.DB.Collection("users")

	// Find company and social indexes
	var user models.User
	err := usersCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return err
	}

	companyIndex := -1
	socialIndex := -1

	for i, comp := range user.Companies {
		if comp.CompanyName == companyName {
			companyIndex = i
			for j, soc := range comp.Socials {
				if soc.Platform == platform {
					socialIndex = j
					break
				}
			}
			break
		}
	}

	if companyIndex == -1 || socialIndex == -1 {
		return mongo.ErrNoDocuments
	}

	// Update using specific array indexes
	updateFields := bson.M{
		fmt.Sprintf("company.%d.socials.%d.access_token", companyIndex, socialIndex): accessToken,
		fmt.Sprintf("company.%d.socials.%d.expires_at", companyIndex, socialIndex):   expiresAt,
	}

	if refreshToken != "" {
		updateFields[fmt.Sprintf("company.%d.socials.%d.refresh_token", companyIndex, socialIndex)] = refreshToken
	}

	update := bson.M{"$set": updateFields}

	result, err := usersCollection.UpdateOne(ctx, bson.M{"_id": userID}, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	log.Printf("Successfully updated tokens for platform %s in company %s", platform, companyName)
	return nil
}
