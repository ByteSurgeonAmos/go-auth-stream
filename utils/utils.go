package utils

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TimeoutWindow(secs int)(context.Context,context.CancelFunc){
	return context.WithTimeout(context.Background(), time.Duration(secs)*time.Second)

}
func CreateJwtToken(userID string, username string, email string)(string, error){
	claims := jwt.MapClaims{
		"user_id": userID,
		"username": username,
		"email": email,
		"exp": time.Now().Add(24 * time.Hour).Unix(),

	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == ""{
		return "", fmt.Errorf("JWT_SECRET environment variable not set")
	}
	tokenString, err := token.SignedString([]byte(jwtSecret))
	return tokenString, err
}
func ValidateJwtToken(tokenString string)(*jwt.Token, error){
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == ""{
		return nil, fmt.Errorf("JWT_SECRET environment variable not set")
	}
	token,err := jwt.Parse(tokenString, func(token *jwt.Token)(interface{}, error){
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok{
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	if err != nil{
		return nil, err
	}
	if !token.Valid{
		return nil, fmt.Errorf("Invalid token")
	}
	return token, nil
}