package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

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

var subscriptionsCollection *mongo.Collection
var plansCollection *mongo.Collection
var paymentUsersCollection *mongo.Collection

func InitSubscriptionHandler() {
	subscriptionsCollection = db.DB.Collection("subscriptions")
	plansCollection = db.DB.Collection("plans")
	paymentUsersCollection = db.DB.Collection("users")
}

func GetAllPlans(c *gin.Context) {
	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	cursor, err := plansCollection.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching plans"})
		return
	}
	defer cursor.Close(ctx)

	var plans []models.Plan
	err = cursor.All(ctx, &plans)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error decoding plans"})
		return
	}

	if plans == nil {
		plans = []models.Plan{}
	}

	c.JSON(http.StatusOK, gin.H{
		"plans": plans,
		"count": len(plans),
	})
}

func CreatePlan(c *gin.Context) {
	var input struct {
		Name        string          `json:"name" binding:"required"`
		PlanCode    string          `json:"plan_code"` 
		Description string          `json:"description"`
		Amount      int             `json:"amount" binding:"required"` 
		Interval    models.Interval `json:"interval" binding:"required"`
		Currency    string          `json:"currency"`
	}

	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validIntervals := map[models.Interval]bool{
		models.IntervalMonthly: true,
		models.IntervalAnnual:  true,
		models.IntervalFree:    true,
	}
	if !validIntervals[input.Interval] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid interval. Must be 'monthly', 'annually', or 'free'"})
		return
	}

	if input.Interval == models.IntervalFree && input.Amount != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Free plans must have amount of 0"})
		return
	}

	planCode := input.PlanCode
	if planCode == "" {
		planCode = utils.GeneratePlanCode(input.Name, string(input.Interval))
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	var existingPlan models.Plan
	err = plansCollection.FindOne(ctx, bson.M{"plan_code": planCode}).Decode(&existingPlan)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Plan with this plan_code already exists",
			"existing_plan_code": planCode,
			"suggestion": "Please provide a different plan_code or let the system generate one automatically",
		})
		return
	}

	currency := input.Currency
	if currency == "" {
		currency = "USD"
	}

	plan := models.Plan{
		ID:          primitive.NewObjectID().Hex(),
		Name:        input.Name,
		PlanCode:    planCode,
		Description: input.Description,
		Amount:      input.Amount,
		Interval:    input.Interval,
		Currency:    currency,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	result, err := plansCollection.InsertOne(ctx, plan)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating plan"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Plan created successfully",
		"plan_id": result.InsertedID,
		"plan":    plan,
	})
}

func UpdatePlan(c *gin.Context) {
	planID := c.Param("plan_id")
	if planID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plan ID is required"})
		return
	}

	var input struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Amount      int             `json:"amount"`
		Interval    models.Interval `json:"interval"`
		Currency    string          `json:"currency"`
	}

	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	var existingPlan models.Plan
	err = plansCollection.FindOne(ctx, bson.M{"_id": planID}).Decode(&existingPlan)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching plan"})
		return
	}

	updateFields := bson.M{
		"updated_at": time.Now(),
	}

	if input.Name != "" {
		updateFields["name"] = input.Name
	}
	if input.Description != "" {
		updateFields["description"] = input.Description
	}
	if input.Amount < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Amount cannot be negative"})
		return
	}
	if input.Amount >= 0 {
		updateFields["amount"] = input.Amount
	}
	if input.Interval != "" {
		updateFields["interval"] = input.Interval
	}
	if input.Currency != "" {
		updateFields["currency"] = input.Currency
	}

	result, err := plansCollection.UpdateOne(
		ctx,
		bson.M{"_id": planID},
		bson.M{"$set": updateFields},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating plan"})
		return
	}

	if result.MatchedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
		return
	}

	var updatedPlan models.Plan
	err = plansCollection.FindOne(ctx, bson.M{"_id": planID}).Decode(&updatedPlan)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Plan updated successfully"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plan updated successfully",
		"plan":    updatedPlan,
	})
}

func DeletePlan(c *gin.Context) {
	planID := c.Param("plan_id")
	if planID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plan ID is required"})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	count, err := subscriptionsCollection.CountDocuments(ctx, bson.M{
		"plan_id": planID,
		"status":  models.StatusActive,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking subscriptions"})
		return
	}

	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Cannot delete plan with %d active subscriptions", count),
		})
		return
	}

	result, err := plansCollection.DeleteOne(ctx, bson.M{"_id": planID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error deleting plan"})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plan deleted successfully",
	})
}

func GetPlanByID(c *gin.Context) {
	planID := c.Param("plan_id")
	if planID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plan ID is required"})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	var plan models.Plan
	err := plansCollection.FindOne(ctx, bson.M{"_id": planID}).Decode(&plan)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching plan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plan": plan,
	})
}

func CreateSubscription(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	var input types.CreateSubscriptionInput
	err := c.ShouldBindJSON(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	var plan models.Plan
	err = plansCollection.FindOne(ctx, bson.M{"_id": input.PlanID}).Decode(&plan)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching plan"})
		return
	}

	var existingSub models.Subscription
	err = subscriptionsCollection.FindOne(ctx, bson.M{
		"user_id": userID,
		"status":  models.StatusActive,
	}).Decode(&existingSub)

	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "User already has an active subscription",
			"subscription": existingSub,
		})
		return
	}

	startDate := time.Now()
	var endDate time.Time
	
	switch plan.Interval {
	case models.IntervalMonthly:
		endDate = startDate.AddDate(0, 1, 0)
	case models.IntervalAnnual:
		endDate = startDate.AddDate(1, 0, 0) 
	case models.IntervalFree:
		endDate = startDate.AddDate(100, 0, 0) 
	default:
		endDate = startDate.AddDate(0, 1, 0) 
	}

	subscription := models.Subscription{
		ID:        primitive.NewObjectID().Hex(),
		UserID:    userID,
		PlanID:    input.PlanID,
		Status:    models.StatusActive,
		StartDate: startDate.Format(time.RFC3339),
		EndDate:   endDate.Format(time.RFC3339),
	}

	result, err := subscriptionsCollection.InsertOne(ctx, subscription)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating subscription"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Subscription created successfully",
		"subscription_id": result.InsertedID,
		"subscription":   subscription,
		"plan":           plan,
	})
}

func GetUserSubscription(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	var subscription models.Subscription
	err := subscriptionsCollection.FindOne(ctx, bson.M{
		"user_id": userID,
		"status":  models.StatusActive,
	}).Decode(&subscription)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "No active subscription found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching subscription"})
		return
	}

	var plan models.Plan
	err = plansCollection.FindOne(ctx, bson.M{"_id": subscription.PlanID}).Decode(&plan)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"subscription": subscription})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscription": subscription,
		"plan":         plan,
	})
}

func InitializePayment(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
		return
	}

	var input types.CreateSubscriptionInput
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
	err = paymentUsersCollection.FindOne(ctx, bson.M{"_id": userObjID}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var plan models.Plan
	err = plansCollection.FindOne(ctx, bson.M{"_id": input.PlanID}).Decode(&plan)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching plan"})
		return
	}

	if plan.Amount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Free plans don't require payment"})
		return
	}

	reference := utils.GenerateTransactionReference(userID)

	callbackURL := os.Getenv("PAYMENT_CALLBACK_URL")
	if callbackURL == "" {
		callbackURL = os.Getenv("APP_URL") + "/api/payments/verify"
	}

	response, err := utils.InitializeTransaction(user.Email, plan.Amount*100, reference, callbackURL)
	if err != nil {
		log.Printf("Paystack initialization error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "Payment initialized successfully",
		"authorization_url": response.Data.AuthorizationURL,
		"access_code":       response.Data.AccessCode,
		"reference":         response.Data.Reference,
		"plan":              plan,
	})
}

func VerifyPayment(c *gin.Context) {
	reference := c.Query("reference")
	if reference == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Transaction reference is required"})
		return
	}

	verifyResponse, err := utils.VerifyTransaction(reference)
	if err != nil {
		log.Printf("Paystack verification error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify payment"})
		return
	}

	if verifyResponse.Data.Status != "success" {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error":   "Payment not successful",
			"status":  verifyResponse.Data.Status,
			"message": verifyResponse.Data.GatewayResponse,
		})
		return
	}

	ctx, cancel := utils.TimeoutWindow(10)
	defer cancel()

	var user models.User
	err = paymentUsersCollection.FindOne(ctx, bson.M{"email": verifyResponse.Data.Customer.Email}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var plan models.Plan
	err = plansCollection.FindOne(ctx, bson.M{"amount": verifyResponse.Data.Amount / 100}).Decode(&plan)
	if err != nil {
		log.Printf("Plan not found for amount: %d", verifyResponse.Data.Amount/100)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Plan not found"})
		return
	}

	var existingSub models.Subscription
	err = subscriptionsCollection.FindOne(ctx, bson.M{
		"user_id": user.ID.Hex(),
		"status":  models.StatusActive,
	}).Decode(&existingSub)

	if err == nil {
		_, err = subscriptionsCollection.UpdateOne(ctx,
			bson.M{"_id": existingSub.ID},
			bson.M{"$set": bson.M{"status": models.StatusCanceled}},
		)
		if err != nil {
			log.Printf("Error canceling existing subscription: %v", err)
		}
	}

	startDate := time.Now()
	var endDate time.Time

	switch plan.Interval {
	case models.IntervalMonthly:
		endDate = startDate.AddDate(0, 1, 0)
	case models.IntervalAnnual:
		endDate = startDate.AddDate(1, 0, 0)
	default:
		endDate = startDate.AddDate(0, 1, 0)
	}

	subscription := models.Subscription{
		ID:        primitive.NewObjectID().Hex(),
		UserID:    user.ID.Hex(),
		PlanID:    plan.ID,
		Status:    models.StatusActive,
		StartDate: startDate.Format(time.RFC3339),
		EndDate:   endDate.Format(time.RFC3339),
	}

	_, err = subscriptionsCollection.InsertOne(ctx, subscription)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating subscription"})
		return
	}

	amountStr := fmt.Sprintf("$%.2f", float64(plan.Amount)/100)
	go func() {
		if err := utils.SendSubscriptionConfirmation(user.Email, plan.Name, amountStr); err != nil {
			log.Printf("Failed to send subscription confirmation email: %v", err)
		}
		if err := utils.SendPaymentReceipt(user.Email, reference, amountStr, plan.Name); err != nil {
			log.Printf("Failed to send payment receipt email: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message":      "Payment verified and subscription activated",
		"subscription": subscription,
		"plan":         plan,
		"transaction":  verifyResponse.Data,
	})
}
