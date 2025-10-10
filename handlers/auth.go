package handlers

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ByteSurgeonAmos/go-auth-stream/connectors"
	"github.com/ByteSurgeonAmos/go-auth-stream/internal/db"
	"github.com/ByteSurgeonAmos/go-auth-stream/models"
	"github.com/ByteSurgeonAmos/go-auth-stream/types"
	"github.com/ByteSurgeonAmos/go-auth-stream/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

var usersCollection *mongo.Collection

func InitAuthHandler() {
	usersCollection = db.DB.Collection("users")
	fmt.Println("Users collection initialized:", usersCollection.Name())
}

func getEnvDuration(key string, defaultMinutes int) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return time.Duration(defaultMinutes) * time.Minute
	}
	minutes, err := strconv.Atoi(value)
	if err != nil {
		return time.Duration(defaultMinutes) * time.Minute
	}
	return time.Duration(minutes) * time.Minute
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

	verificationCode := generate2FACode()
	verificationCodeExpiry := getEnvDuration("VERIFICATION_CODE_EXPIRY", 10)
	verificationExpiry := time.Now().Add(verificationCodeExpiry)

	user := models.User{
		UserName:           input.UserName,
		Email:              input.Email,
		Password:           hashedPassword,
		Role:               models.RoleUser,
		IsActive:           false, 
		VerificationCode:   verificationCode,
		VerificationExpiry: verificationExpiry,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	result, err := usersCollection.InsertOne(ctx, user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}

	go func() {
		if err := utils.SendVerificationCode(input.Email, verificationCode); err != nil {
			log.Printf("Failed to send verification code to %s: %v", input.Email, err)
		}
	}()

	c.JSON(http.StatusCreated, gin.H{
		"message": "Account created successfully. Please check your email for verification code.",
		"user_id": result.InsertedID,
		"requires_verification": true,
	})
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

	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account not verified. Please verify your account first."})
		return
	}

	isValid, err := CheckPasswordHash(input.Password, user.Password)
	if !isValid || err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	code := generate2FACode()
	twoFactorExpiry := getEnvDuration("TWO_FACTOR_CODE_EXPIRY", 10)
	expiry := time.Now().Add(twoFactorExpiry)

	filter := bson.M{"email": input.Email}
	update := bson.M{
		"$set": bson.M{
			"two_factor_code":   code,
			"two_factor_expiry": expiry,
		},
	}
	_, err = usersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating 2FA code"})
		return
	}

	go func() {
		if err := utils.Send2FACode(user.Email, code); err != nil {
			log.Printf("Failed to send 2FA code to %s: %v", user.Email, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message":      "2FA code sent to your email. Please verify to complete login.",
		"requires_2fa": true,
	})
}

func Verify2FA(c *gin.Context) {
	var input types.TwoFactorVerifyInput
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid request"})
		return
	}

	if user.TwoFactorCode != input.Code {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid verification code"})
		return
	}

	if time.Now().After(user.TwoFactorExpiry) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Verification code has expired"})
		return
	}

	role := string(user.Role)
	if role == "" {
		role = string(models.RoleUser)
	}

	token, err := utils.CreateJwtToken(user.ID.Hex(), user.UserName, user.Email, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		return
	}

	refreshToken, err := utils.CreateRefreshToken(user.ID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating refresh token"})
		return
	}

	refreshTokenExpiry := getEnvDuration("REFRESH_TOKEN_EXPIRY", 10080)
	filter := bson.M{"email": input.Email}
	update := bson.M{
		"$unset": bson.M{
			"two_factor_code":   "",
			"two_factor_expiry": "",
		},
		"$set": bson.M{
			"refresh_token":        refreshToken,
			"refresh_token_expiry": time.Now().Add(refreshTokenExpiry),
			"updated_at":           time.Now(),
		},
	}
	_, err = usersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Printf("Error updating tokens: %v", err)
	}

	accessTokenExpiryMinutes := getEnvDuration("ACCESS_TOKEN_EXPIRY", 15)
	c.JSON(http.StatusOK, gin.H{
		"message":       "2FA verification successful",
		"user_id":       user.ID,
		"access_token":  token,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(accessTokenExpiryMinutes.Seconds()),
		"role":          role,
	})
}

func generate2FACode() string {
	rand.Seed(time.Now().UnixNano())
	code := rand.Intn(900000) + 100000
	return fmt.Sprintf("%06d", code)
}

func VerifyAccount(c *gin.Context) {
	var input types.VerifyAccountInput
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid request"})
		return
	}

	if user.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account is already verified"})
		return
	}

	if user.VerificationCode != input.Code {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid verification code"})
		return
	}

	if time.Now().After(user.VerificationExpiry) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Verification code has expired. Please request a new one."})
		return
	}

	role := string(user.Role)
	if role == "" {
		role = string(models.RoleUser)
	}

	token, err := utils.CreateJwtToken(user.ID.Hex(), user.UserName, user.Email, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		return
	}

	refreshToken, err := utils.CreateRefreshToken(user.ID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating refresh token"})
		return
	}

	refreshTokenExpiry := getEnvDuration("REFRESH_TOKEN_EXPIRY", 10080)
	filter := bson.M{"email": input.Email}
	update := bson.M{
		"$set": bson.M{
			"is_active":            true,
			"refresh_token":        refreshToken,
			"refresh_token_expiry": time.Now().Add(refreshTokenExpiry),
			"updated_at":           time.Now(),
		},
		"$unset": bson.M{
			"verification_code":   "",
			"verification_expiry": "",
		},
	}
	_, err = usersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error activating account"})
		return
	}

	accessTokenExpiryMinutes := getEnvDuration("ACCESS_TOKEN_EXPIRY", 15)
	c.JSON(http.StatusOK, gin.H{
		"message":       "Account verified successfully. You are now logged in.",
		"access_token":  token,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(accessTokenExpiryMinutes.Seconds()),
		"user_id":       user.ID,
		"email":         user.Email,
		"username":      user.UserName,
		"role":          role,
	})
}

func ResendVerificationCode(c *gin.Context) {
	var input struct {
		Email string `json:"email" binding:"required,email"`
	}
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
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Account is already verified"})
		return
	}

	verificationCode := generate2FACode()
	verificationCodeExpiry := getEnvDuration("VERIFICATION_CODE_EXPIRY", 10)
	verificationExpiry := time.Now().Add(verificationCodeExpiry)

	filter := bson.M{"email": input.Email}
	update := bson.M{
		"$set": bson.M{
			"verification_code":   verificationCode,
			"verification_expiry": verificationExpiry,
			"updated_at":          time.Now(),
		},
	}
	_, err = usersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating verification code"})
		return
	}

	go func() {
		if err := utils.SendVerificationCode(input.Email, verificationCode); err != nil {
			log.Printf("Failed to send verification code to %s: %v", input.Email, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Verification code sent to your email.",
	})
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

	var user models.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": input.UserID}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching updated user"})
		return
	}

	companyIndex := len(user.Companies) - 1 

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

func InitiateFacebookAuth(c *gin.Context) {
	state := utils.GenerateRandomState()
	utils.StoreOAuthState(state)
	authURL := connectors.GetFacebookAuthURL(state)
	
	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

func FacebookCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code not provided"})
		return
	}
	
	if !utils.ValidateOAuthState(state) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state parameter"})
		return
	}
	
	ctx, cancel := utils.TimeoutWindow(30)
	defer cancel()
	
	token, err := connectors.ExchangeFacebookCodeForToken(ctx, code)
	if err != nil {
		log.Printf("Failed to exchange Facebook code for token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to authenticate with Facebook"})
		return
	}
	
	pages, err := connectors.GetFacebookPages(ctx, token.AccessToken)
	if err != nil {
		log.Printf("Failed to get Facebook pages: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Facebook pages"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":       "Facebook authentication successful",
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expires_at":    token.Expiry,
		"pages":         pages,
		"state":         state,
	})
}

func InitiateInstagramAuth(c *gin.Context) {
	state := utils.GenerateRandomState()
	utils.StoreOAuthState(state)
	authURL := connectors.GetInstagramAuthURL(state)
	
	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

func InstagramCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code not provided"})
		return
	}
	
	if !utils.ValidateOAuthState(state) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state parameter"})
		return
	}
	
	ctx, cancel := utils.TimeoutWindow(30)
	defer cancel()
	
	token, err := connectors.ExchangeInstagramCodeForToken(ctx, code)
	if err != nil {
		log.Printf("Failed to exchange Instagram code for token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to authenticate with Instagram"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":       "Instagram authentication successful",
		"access_token":  token.AccessToken,
		"expires_at":    token.Expiry,
		"state":         state,
	})
}

func InitiateTwitterAuth(c *gin.Context) {
	state := utils.GenerateRandomState()
	utils.StoreOAuthState(state)
	authURL := connectors.GetTwitterAuthURL(state)
	
	c.JSON(http.StatusOK, gin.H{
		"auth_url": authURL,
		"state":    state,
	})
}

func TwitterCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code not provided"})
		return
	}
	
	if !utils.ValidateOAuthState(state) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state parameter"})
		return
	}
	
	ctx, cancel := utils.TimeoutWindow(30)
	defer cancel()
	
	token, err := connectors.ExchangeCodeForToken(ctx, code)
	if err != nil {
		log.Printf("Failed to exchange Twitter code for token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to authenticate with Twitter"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message":       "Twitter authentication successful",
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"expires_at":    token.Expiry,
		"state":         state,
	})
}

func RefreshToken(c *gin.Context) {
	var input types.RefreshTokenInput
	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := utils.ValidateRefreshToken(input.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	var user models.User
	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = usersCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is not active"})
		return
	}

	if user.RefreshToken != input.RefreshToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	if time.Now().After(user.RefreshTokenExpiry) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh token has expired. Please login again."})
		return
	}

	role := string(user.Role)
	if role == "" {
		role = string(models.RoleUser)
	}

	newAccessToken, err := utils.CreateJwtToken(user.ID.Hex(), user.UserName, user.Email, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating access token"})
		return
	}

	newRefreshToken, err := utils.CreateRefreshToken(user.ID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating refresh token"})
		return
	}

	refreshTokenExpiry := getEnvDuration("REFRESH_TOKEN_EXPIRY", 10080)
	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"refresh_token":        newRefreshToken,
			"refresh_token_expiry": time.Now().Add(refreshTokenExpiry),
			"updated_at":           time.Now(),
		},
	}
	_, err = usersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Printf("Error updating refresh token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating tokens"})
		return
	}

	accessTokenExpiryMinutes := getEnvDuration("ACCESS_TOKEN_EXPIRY", 15)
	c.JSON(http.StatusOK, gin.H{
		"message":       "Token refreshed successfully",
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(accessTokenExpiryMinutes.Seconds()),
	})
}

func RevokeToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refresh token is required"})
		return
	}

	userID, err := utils.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var user models.User
	err = usersCollection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching user"})
		return
	}

	if user.RefreshToken != req.RefreshToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$unset": bson.M{
			"refresh_token":        "",
			"refresh_token_expiry": "",
		},
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}
	_, err = usersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Token revoked successfully",
	})
}

func Logout(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	objectID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$unset": bson.M{
			"refresh_token":        "",
			"refresh_token_expiry": "",
		},
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}
	_, err = usersCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Printf("Error clearing refresh token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error logging out"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}