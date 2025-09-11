package handlers

import (
	"fmt"

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
var usersCollection  *mongo.Collection


func InitAuthHandler(){
	usersCollection = db.DB.Collection("users")
	fmt.Println("Users collection initialized:", usersCollection.Name())
}
func Signup(c *gin.Context){
	var input types.SignupInput
	err := c.ShouldBindJSON(&input)
	if err != nil{
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

 	ctx,cancel :=  utils.TimeoutWindow(10)
	defer cancel()

	count, err := usersCollection.CountDocuments(ctx, bson.M{"email": input.Email})
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists. Kindly login instead."})
		return
	}
	if err != nil{
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking for existing user"})
		return
	}
	
	hashedPassword, err := HashPassword(input.Password)
	if err != nil{
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error hashing password"})
		return
	}
	
	user := models.User{
		UserName: input.UserName,
		Email: input.Email,
		Password: hashedPassword,
	}
	
	result, err := usersCollection.InsertOne(ctx, user)
	if err != nil{
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating user"})
		return
	}
	c.JSON(http.StatusCreated,gin.H{"message": "User created successfully", "user_id": result.InsertedID})

}
func HashPassword(password string)(string, error){
	bytes, err := bcrypt.GenerateFromPassword([]byte (password),bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) (bool, error) {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    if err != nil {
        return false, err
    }
    return true, nil
}


func Login(c *gin.Context){
 var input types.LoginInput
 err := c.ShouldBindJSON(&input)
 if err != nil{
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	return
 }
 ctx,cancel :=  utils.TimeoutWindow(10)
 defer cancel()

var user models.User
err = usersCollection.FindOne(ctx,bson.M{"email":input.Email}).Decode(&user)
if err != nil{
	if err == mongo.ErrNoDocuments{
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching user"})
	return
}
isValid,err := CheckPasswordHash(input.Password, user.Password)
if !isValid || err != nil{
	c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
	return
}
token,err := utils.CreateJwtToken(user.ID.Hex(), user.UserName, user.Email)
if err != nil{
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
	return
}
c.JSON(http.StatusOK, gin.H{"message": "Login successful", "user_id": user.ID, "token": token})

}