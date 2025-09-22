package router

import (
	"net/http"

	"github.com/ByteSurgeonAmos/go-auth-stream/handlers"
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
	}
	
	signUpProcess := r.Group("/api/signup-process")
	{
		signUpProcess.POST("/updateCompany", handlers.UpdateCompany)
	}
	
	agentRoutes := r.Group("/api/agent")
	{
		agentRoutes.GET("/generate/stream", handlers.StreamPostGeneration)
		agentRoutes.POST("/generate/stream", handlers.StreamPostGeneration)
		
		agentRoutes.POST("/generate", handlers.GeneratePost)
		
		agentRoutes.GET("/companies/:user_id", handlers.GetUserCompanies)
		
		agentRoutes.POST("/conversations", handlers.CreateConversation)
	}
	
	return r
}