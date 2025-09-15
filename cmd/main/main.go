package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ByteSurgeonAmos/go-auth-stream/handlers"
	"github.com/ByteSurgeonAmos/go-auth-stream/internal/cron"
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
	defer db.CloseConnection()
	
	handlers.InitAuthHandler()
	
	err = handlers.InitScraperHandler()
	if err != nil {
		log.Printf("Warning: Failed to initialize scraper handler: %v", err)
		log.Println("Scraper endpoints will not be available")
	} else {
		log.Println("Scraper service initialized successfully")
		defer func() {
			if err := handlers.CloseScraperHandler(); err != nil {
				log.Printf("Error closing scraper handler: %v", err)
			}
		}()
	}
	
	var cronService *cron.CronService
	if err == nil { 
		cronService, err = cron.NewCronService()
		if err != nil {
			log.Printf("Warning: Failed to initialize cron service: %v", err)
		} else {
			cronService.Start()
			log.Println("Cron service started for automatic company data refresh")
			defer cronService.Stop()
		}
	}
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	r := router.SetUpRouter()
	
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-c
		fmt.Println("\nGracefully shutting down...")
		if cronService != nil {
			cronService.Stop()
		}
		handlers.CloseScraperHandler()
		db.CloseConnection()
		os.Exit(0)
	}()
	
	fmt.Println("Server running on port:", port)
	r.Run(":" + port)
}