package utils

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
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

func encrypt(plain []byte) (string, error) {
    key := []byte(os.Getenv("AES_KEY"))
    if len(key) != 32 {
        return "", errors.New("AES_KEY must be 32 bytes")
    }
    block, err := aes.NewCipher(key)
    if err != nil { return "", err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return "", err }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return "", err }
    ciphertext := gcm.Seal(nil, nonce, plain, nil)
    payload := append(nonce, ciphertext...)
    return base64.StdEncoding.EncodeToString(payload), nil
}

func decrypt(enc string) ([]byte, error) {
    key := []byte(os.Getenv("AES_KEY"))
    if len(key) != 32 { return nil, errors.New("AES_KEY must be 32 bytes") }
    data, err := base64.StdEncoding.DecodeString(enc)
    if err != nil { return nil, err }
    block, err := aes.NewCipher(key)
    if err != nil { return nil, err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return nil, err }
    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize { return nil, errors.New("invalid data") }
    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}

func IsUnauthorizedError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return Contains(errStr, "401") || Contains(errStr, "Unauthorized") || Contains(errStr, "invalid token")
}

func Contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(hasPrefix(s, substr) || hasSuffix(s, substr) || containsInMiddle(s, substr)))
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}