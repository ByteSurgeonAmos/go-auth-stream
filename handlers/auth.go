package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/ByteSurgeonAmos/go-auth-stream/internal/db"
	"github.com/ByteSurgeonAmos/go-auth-stream/models"
	"github.com/ByteSurgeonAmos/go-auth-stream/types"
	"github.com/ByteSurgeonAmos/go-auth-stream/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var usersCollection *mongo.Collection

func InitAuthHandler() {
	usersCollection = db.DB.Collection("users")
	fmt.Println("Users collection initialized:", usersCollection.Name())
}

func Signup(c *gin.Context) {
	var input types.SignupInput
	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	count, err := usersCollection.CountDocuments(ctx, bson.M{"email": input.Email})
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists. Kindly login instead."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking for existing user"})
		return
	}

	hashedPassword, err := HashPassword(input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error hashing password"})
		return
	}

	user := models.User{
		UserName: input.UserName,
		Email:    input.Email,
		Password: hashedPassword,
	}

	result, err := usersCollection.InsertOne(ctx, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Request processed successfully. Kindly proceed with the sign up process", "user_id": result.InsertedID})

}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return false, err
	}
	return true, nil
}

func Login(c *gin.Context) {
	var input types.LoginInput
	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	var user models.User
	err = usersCollection.FindOne(ctx, bson.M{"email": input.Email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching user"})
		return
	}
	isValid, err := CheckPasswordHash(input.Password, user.Password)
	if !isValid || err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}
	token, err := utils.CreateJwtToken(user.ID.Hex(), user.UserName, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Login successful", "user_id": user.ID, "token": token})

}

func UpdateCompany(c *gin.Context) {
	var input struct {
		UserID  string        `json:"user_id" binding:"required"`
		Company models.Company `json:"company" binding:"required"`
	}
	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	// First, add the company to the user's companies array
	filter := bson.M{"_id": input.UserID}
	update := bson.M{"$push": bson.M{"company": input.Company}}
	result, err := usersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating company information"})
		return
	}
	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get the index of the newly added company
	var user models.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": input.UserID}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching updated user"})
		return
	}

	companyIndex := len(user.Companies) - 1 // Index of the newly added company

	// Trigger async scraping if scraper service is available
	if scraperService != nil && input.Company.Link != "" {
		log.Printf("Triggering scraping for company: %s", input.Company.CompanyName)
		scrapeCompanyAsync(&input.Company, input.UserID, companyIndex)
	} else {
		log.Printf("Scraper service not available or no URL provided for company: %s", input.Company.CompanyName)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            "Company information updated successfully",
		"scraping_initiated": scraperService != nil && input.Company.Link != "",
	})
}