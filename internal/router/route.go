package router

import (
	"net/http"

	"github.com/ByteSurgeonAmos/go-auth-stream/handlers"
	"github.com/ByteSurgeonAmos/go-auth-stream/internal/middleware"
	"github.com/gin-gonic/gin"
)

func SetUpRouter()* gin.Engine{
	r := gin.Default()

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Welcome to the Agent4 API",
		})
	})
	r.GET("/health", func(c *gin.Context){
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
		})
	})
	
	authRoutes := r.Group("/api/auth")
	{
		authRoutes.POST("/signup", handlers.Signup)
		authRoutes.POST("/login", handlers.Login)
		authRoutes.POST("/verify-2fa", handlers.Verify2FA)
		
		authRoutes.GET("/twitter/init", handlers.InitiateTwitterAuth)
		authRoutes.GET("/twitter/callback", handlers.TwitterCallback)
		
		authRoutes.GET("/facebook/init", handlers.InitiateFacebookAuth)
		authRoutes.	GET("/facebook/callback", handlers.FacebookCallback)
		
		authRoutes.GET("/instagram/init", handlers.InitiateInstagramAuth)
		authRoutes.GET("/instagram/callback", handlers.InstagramCallback)
	}
	
	signUpProcess := r.Group("/api/signup-process")
	signUpProcess.Use(middleware.AuthMiddleware())
	{
		signUpProcess.POST("/updateCompany", handlers.UpdateCompany)
	}
	
	socialRoutes := r.Group("/api/social")
	{
		socialRoutes.GET("/platforms", handlers.GetAllPlatforms) 
		
		socialRoutes.PUT("/platforms", middleware.AuthMiddleware(), handlers.UpdateSocialMedia)
		socialRoutes.GET("/user/platforms", middleware.AuthMiddleware(), handlers.GetUserSocialMedia)
	}
	
	postRoutes := r.Group("/api/posts")
	postRoutes.Use(middleware.AuthMiddleware())
	{
		postRoutes.POST("/", handlers.CreatePost)
		postRoutes.GET("/", handlers.GetAllPosts)
		postRoutes.GET("/scheduled", handlers.GetScheduledPosts)
		postRoutes.DELETE("/scheduled/:post_id", handlers.CancelScheduledPost)
		postRoutes.POST("/publish", handlers.PublishPost)
	}
	
	planRoutes := r.Group("/api/plans")
	{
		planRoutes.GET("/", handlers.GetAllPlans)
	}
	
	subscriptionRoutes := r.Group("/api/subscriptions")
	subscriptionRoutes.Use(middleware.AuthMiddleware())
	{
		subscriptionRoutes.POST("/", handlers.CreateSubscription)
		subscriptionRoutes.GET("/", handlers.GetUserSubscription)
	}
	
	paymentRoutes := r.Group("/api/payments")
	{
		paymentRoutes.POST("/initialize", middleware.AuthMiddleware(), handlers.InitializePayment)
		paymentRoutes.GET("/verify", handlers.VerifyPayment) 
	}
	
	agentRoutes := r.Group("/api/agent")
	agentRoutes.Use(middleware.AuthMiddleware())
	{
		agentRoutes.GET("/generate/stream", handlers.StreamPostGeneration)
		agentRoutes.POST("/generate/stream", handlers.StreamPostGeneration)
		
		agentRoutes.POST("/generate", handlers.GeneratePost)
		
		agentRoutes.GET("/companies/:user_id", handlers.GetUserCompanies)
		
		agentRoutes.POST("/conversations", handlers.CreateConversation)
	}
	
	return r
}