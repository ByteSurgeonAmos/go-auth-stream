package main

import (
	"fmt"
	"os"

	"github.com/ByteSurgeonAmos/go-auth-stream/handlers"
	"github.com/ByteSurgeonAmos/go-auth-stream/internal/db"
	"github.com/ByteSurgeonAmos/go-auth-stream/internal/router"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil{
		fmt.Println("Error loading .env file, proceeding with system environment variables")
	}
	db.ConnectDB()
	handlers.InitAuthHandler()
	defer db.CloseConnection()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r := router.SetUpRouter()
	fmt.Println("Server running on port:", port)
	r.Run(":" + port)
}