package handlers

import (
	"fmt"
	"net/http"

	"github.com/ByteSurgeonAmos/go-auth-stream/internal/db"
	"github.com/ByteSurgeonAmos/go-auth-stream/internal/middleware"
	"github.com/ByteSurgeonAmos/go-auth-stream/models"
	"github.com/ByteSurgeonAmos/go-auth-stream/types"
	"github.com/ByteSurgeonAmos/go-auth-stream/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var socialUsersCollection *mongo.Collection

func InitSocialHandler() {
	socialUsersCollection = db.DB.Collection("users")
}

func GetAllPlatforms(c *gin.Context) {
	platforms := []gin.H{
		{
			"platform": string(models.PlatformX),
			"name":     "X (Twitter)",
		},
		{
			"platform": string(models.PlatformFacebook),
			"name":     "Facebook",
		},
		{
			"platform": string(models.PlatformInstagram),
			"name":     "Instagram",
		},
	}
	c.JSON(http.StatusOK, gin.H{"platforms": platforms})
}

func UpdateSocialMedia(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	var input types.UpdateSocialMediaInput
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

	var user models.User
	err = socialUsersCollection.FindOne(ctx, bson.M{"_id": userObjID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching user"})
		return
	}

	companyIndex := -1
	for i, company := range user.Companies {
		if company.CompanyName == input.CompanyName {
			companyIndex = i
			break
		}
	}

	if companyIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Company not found"})
		return
	}

	fieldPath := fmt.Sprintf("company.%d.socials", companyIndex)
	
	filter := bson.M{"_id": userObjID}
	update := bson.M{
		"$push": bson.M{
			fieldPath: input.Social,
		},
	}

	result, err := socialUsersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating social media platform"})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Social media platform updated successfully"})
}

func GetUserSocialMedia(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	companyName := c.Query("company_name")

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	err = socialUsersCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching user"})
		return
	}

	if companyName != "" {
		for _, company := range user.Companies {
			if company.CompanyName == companyName {
				c.JSON(http.StatusOK, gin.H{
					"company": company.CompanyName,
					"socials": company.Socials,
				})
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Company not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"companies": user.Companies})
}
