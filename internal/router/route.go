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
			"message": "Welcome to the Go Auth Stream API",
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

	return r
	
}