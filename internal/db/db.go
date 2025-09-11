package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)
var Client *mongo.Client
var DB *mongo.Database
func ConnectDB() *mongo.Database{
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file:", err)
		fmt.Println("Proceeding with system environment variables")
	}
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		panic("MONGODB_URI environment variable not set")
	}
	client,err := mongo.NewClient(options.Client().ApplyURI(mongoURI))
	if err != nil{
		log.Fatal("Error creating MongoDB client:", err)
	}
	ctx,cancel := context.WithTimeout(context.Background(),10*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil{
		log.Fatal("Error connecting to MongoDB:", err)
	}

	err = client.Ping(ctx,nil)
	if err != nil{
		log.Fatal("Error pinging MongoDB:", err)
	}
	fmt.Println("Connected to MongoDB!")
	DB = client.Database("auth-streaming")
	Client = client
	return DB
}
func CloseConnection(){
	if Client != nil{
		ctx, cancel := context.WithTimeout(context.Background(), 10 *time.Second)
		defer cancel()
		_ = Client.Disconnect(ctx)
		fmt.Println("Disconnected from MongoDB")
	}
}

