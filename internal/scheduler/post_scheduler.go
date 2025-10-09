package scheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/ByteSurgeonAmos/go-auth-stream/handlers"
	"github.com/ByteSurgeonAmos/go-auth-stream/internal/db"
	"github.com/ByteSurgeonAmos/go-auth-stream/models"
	"github.com/ByteSurgeonAmos/go-auth-stream/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type PostScheduler struct {
	postsCollection *mongo.Collection
	usersCollection *mongo.Collection
	stopChan        chan bool
	isRunning       bool
}

func NewPostScheduler() *PostScheduler {
	return &PostScheduler{
		postsCollection: db.DB.Collection("posts"),
		usersCollection: db.DB.Collection("users"),
		stopChan:        make(chan bool),
		isRunning:       false,
	}
}

func (ps *PostScheduler) Start() {
	if ps.isRunning {
		log.Println("Scheduler is already running")
		return
	}

	ps.isRunning = true
	log.Println("Post scheduler started")

	go ps.processScheduledPosts()

	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				ps.processScheduledPosts()
			case <-ps.stopChan:
				ticker.Stop()
				log.Println("Scheduler stopped")
				return
			}
		}
	}()
}

func (ps *PostScheduler) Stop() {
	if ps.isRunning {
		ps.stopChan <- true
		ps.isRunning = false
	}
}

func (ps *PostScheduler) processScheduledPosts() {
	ctx, cancel := utils.TimeoutWindow(60)
	defer cancel()

	filter := bson.M{
		"status": "scheduled",
		"scheduled_at": bson.M{
			"$lte": time.Now(),
		},
	}

	cursor, err := ps.postsCollection.Find(ctx, filter)
	if err != nil {
		log.Printf("Error finding scheduled posts: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var posts []models.Post
	if err := cursor.All(ctx, &posts); err != nil {
		log.Printf("Error decoding scheduled posts: %v", err)
		return
	}

	if len(posts) == 0 {
		return
	}

	log.Printf("Found %d scheduled posts to publish", len(posts))

	for _, post := range posts {
		go ps.publishPost(post)
	}
}

func (ps *PostScheduler) publishPost(post models.Post) {
	ctx, cancel := utils.TimeoutWindow(60)
	defer cancel()

	log.Printf("Publishing scheduled post: %s for user: %s", post.ID.Hex(), post.UserID.Hex())

	_, err := ps.postsCollection.UpdateOne(ctx, bson.M{"_id": post.ID}, bson.M{
		"$set": bson.M{
			"status": "publishing",
		},
	})
	if err != nil {
		log.Printf("Error updating post status to publishing: %v", err)
		return
	}

	var user models.User
	err = ps.usersCollection.FindOne(ctx, bson.M{"_id": post.UserID}).Decode(&user)
	if err != nil {
		ps.markPostAsFailed(post.ID, "User not found")
		return
	}

	var company *models.Company
	for _, comp := range user.Companies {
		if comp.CompanyName == post.CompanyName {
			company = &comp
			break
		}
	}

	if company == nil {
		ps.markPostAsFailed(post.ID, "Company not found")
		return
	}

	results := make(map[string]interface{})
	allSucceeded := true

	for _, platformName := range post.Platforms {
		platform := models.Platform(platformName)

		var social *models.Social
		for _, s := range company.Socials {
			if s.Platform == platform {
				social = &s
				break
			}
		}

		if social == nil {
			results[platformName] = map[string]interface{}{
				"success": false,
				"error":   "Platform not connected",
			}
			allSucceeded = false
			continue
		}

		var postID string
		var publishErr error

		switch platform {
		case models.PlatformX:
			postID, publishErr = handlers.PublishToTwitter(ctx, post.UserID, post.CompanyName, social, post.Content)
		case models.PlatformFacebook:
			postID, publishErr = handlers.PublishToFacebook(ctx, post.UserID, post.CompanyName, social, post.Content)
		case models.PlatformInstagram:
			if post.ImageURL == "" {
				results[platformName] = map[string]interface{}{
					"success": false,
					"error":   "Image URL required for Instagram",
				}
				allSucceeded = false
				continue
			}
			postID, publishErr = handlers.PublishToInstagram(ctx, post.UserID, post.CompanyName, social, post.Content, post.ImageURL)
		}

		if publishErr != nil {
			results[platformName] = map[string]interface{}{
				"success": false,
				"error":   publishErr.Error(),
			}
			allSucceeded = false
		} else {
			results[platformName] = map[string]interface{}{
				"success": true,
				"post_id": postID,
			}
		}
	}

	if allSucceeded {
		_, err = ps.postsCollection.UpdateOne(ctx, bson.M{"_id": post.ID}, bson.M{
			"$set": bson.M{
				"status":       "published",
				"published_at": time.Now(),
				"error_msg":    "",
			},
		})
		log.Printf("Successfully published scheduled post: %s", post.ID.Hex())
	} else {
		retryCount := post.RetryCount + 1
		maxRetries := 3

		if retryCount < maxRetries {
			newScheduledAt := time.Now().Add(5 * time.Minute)
			_, err = ps.postsCollection.UpdateOne(ctx, bson.M{"_id": post.ID}, bson.M{
				"$set": bson.M{
					"status":       "scheduled",
					"scheduled_at": newScheduledAt,
					"retry_count":  retryCount,
					"error_msg":    fmt.Sprintf("Retry %d/%d - Partial failure", retryCount, maxRetries),
				},
			})
			log.Printf("Rescheduled post %s for retry %d/%d at %s", post.ID.Hex(), retryCount, maxRetries, newScheduledAt)
		} else {
			errorMsg := fmt.Sprintf("Failed after %d retries. Results: %v", maxRetries, results)
			ps.markPostAsFailed(post.ID, errorMsg)
		}
	}

	if err != nil {
		log.Printf("Error updating post status: %v", err)
	}
}

func (ps *PostScheduler) markPostAsFailed(postID primitive.ObjectID, errorMsg string) {
	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	_, err := ps.postsCollection.UpdateOne(ctx, bson.M{"_id": postID}, bson.M{
		"$set": bson.M{
			"status":    "failed",
			"error_msg": errorMsg,
		},
	})
	if err != nil {
		log.Printf("Error marking post as failed: %v", err)
	}
	log.Printf("Post %s marked as failed: %s", postID.Hex(), errorMsg)
}

func (ps *PostScheduler) GetScheduledPostsCount() (int64, error) {
	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	count, err := ps.postsCollection.CountDocuments(ctx, bson.M{"status": "scheduled"})
	return count, err
}
