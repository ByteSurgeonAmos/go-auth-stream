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
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/oauth2"
)

var postsCollection *mongo.Collection

func InitPostHandler() {
	postsCollection = db.DB.Collection("posts")
}

func validateUserCompanyAndPlatforms(ctx context.Context, userID primitive.ObjectID, companyName string, platforms []string) (bool, string, error) {
	var user models.User
	err := db.DB.Collection("users").FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if (err != nil) {
		return false, "", fmt.Errorf("error fetching user: %v", err)
	}

	var targetCompany *models.Company
	for _, comp := range user.Companies {
		if comp.CompanyName == companyName {
			targetCompany = &comp
			break
		}
	}

	if targetCompany == nil {
		return false, fmt.Sprintf("Company '%s' not found. Please add this company first.", companyName), nil
	}

	linkedPlatforms := make(map[string]bool)
	for _, social := range targetCompany.Socials {
		linkedPlatforms[string(social.Platform)] = true
	}

	var unlinkedPlatforms []string
	for _, platform := range platforms {
		if !linkedPlatforms[platform] {
			unlinkedPlatforms = append(unlinkedPlatforms, platform)
		}
	}

	if len(unlinkedPlatforms) > 0 {
		return false, fmt.Sprintf("The following platforms are not linked to company '%s': %v. Please link these platforms before creating posts.", companyName, unlinkedPlatforms), nil
	}

	return true, "", nil
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

	valid, errorMsg, err := validateUserCompanyAndPlatforms(ctx, userObjID, input.CompanyName, input.Platforms)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": errorMsg,
			"hint": "Use GET /api/social/user/platforms to see your linked platforms",
		})
		return
	}

	status := "draft"
	var scheduledAt time.Time
	if input.ScheduledAt != nil {
		status = "scheduled"
		scheduledAt = *input.ScheduledAt
		
		if scheduledAt.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Scheduled time must be in the future"})
			return
		}
	}

	post := models.Post{
		ID:          primitive.NewObjectID(),
		Content:     input.Content,
		Images:      input.Images,
		CreatedAt:   time.Now(),
		ScheduledAt: scheduledAt,
		Status:      status,
		UserID:      userObjID,
		CompanyName: input.CompanyName,
		Platforms:   input.Platforms,
		ImageURL:    input.ImageURL,
		RetryCount:  0,
	}

	result, err := postsCollection.InsertOne(ctx, post)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating post"})
		return
	}

	message := "Post created successfully"
	if status == "scheduled" {
		message = fmt.Sprintf("Post scheduled for %s", scheduledAt.Format(time.RFC3339))
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": message,
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

func GetScheduledPosts(c *gin.Context) {
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

	filter := bson.M{
		"user_id": objID,
		"status":  "scheduled",
	}

	sortOptions := bson.D{{Key: "scheduled_at", Value: 1}}
	cursor, err := postsCollection.Find(ctx, filter, options.Find().SetSort(sortOptions))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching scheduled posts"})
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

func CancelScheduledPost(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	postID := c.Param("post_id")
	postObjID, err := primitive.ObjectIDFromHex(postID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	userObjID, _ := primitive.ObjectIDFromHex(userID)

	result, err := postsCollection.UpdateOne(
		ctx,
		bson.M{
			"_id":     postObjID,
			"user_id": userObjID,
			"status":  "scheduled",
		},
		bson.M{
			"$set": bson.M{
				"status": "draft",
			},
		},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error cancelling scheduled post"})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Scheduled post not found or already published"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Scheduled post cancelled successfully",
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

	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	valid, errorMsg, err := validateUserCompanyAndPlatforms(ctx, userObjID, input.CompanyName, input.Platforms)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": errorMsg,
			"hint": "Use GET /api/social/user/platforms to see your linked platforms",
		})
		return
	}

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

func PublishToTwitter(ctx context.Context, userID primitive.ObjectID, companyName string, social *models.Social, content string) (string, error) {
	return publishToTwitter(ctx, userID, companyName, social, content)
}

func PublishToFacebook(ctx context.Context, userID primitive.ObjectID, companyName string, social *models.Social, content string) (string, error) {
	return publishToFacebook(ctx, userID, companyName, social, content)
}

func PublishToInstagram(ctx context.Context, userID primitive.ObjectID, companyName string, social *models.Social, caption, imageURL string) (string, error) {
	return publishToInstagram(ctx, userID, companyName, social, caption, imageURL)
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

	postID, newToken, err := connectors.PostToFacebookWithRetry(ctx, token, social.Username, content)
	if err != nil {
		return "", err
	}

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

	postID, newToken, err := connectors.PostToInstagramWithRetry(ctx, token, social.Username, caption, imageURL)
	if err != nil {
		return "", err
	}

	if newToken.AccessToken != token.AccessToken {
		if err := UpdateSocialToken(userID, companyName, social.Platform, newToken.AccessToken, "", newToken.Expiry); err != nil {
			log.Printf("Warning: Failed to update Instagram token in database: %v", err)
		}
	}

	return postID, nil
}

func UpdateSocialToken(userID primitive.ObjectID, companyName string, platform models.Platform, accessToken, refreshToken string, expiresAt time.Time) error {
	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	usersCollection := db.DB.Collection("users")

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
