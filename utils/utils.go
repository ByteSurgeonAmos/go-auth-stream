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
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	oauthStates = make(map[string]time.Time)
	stateMutex  sync.RWMutex
)

func getEnvDuration(key string, defaultMinutes int) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return time.Duration(defaultMinutes) * time.Minute
	}
	minutes, err := strconv.Atoi(value)
	if err != nil {
		return time.Duration(defaultMinutes) * time.Minute
	}
	return time.Duration(minutes) * time.Minute
}

func TimeoutWindow(secs int)(context.Context,context.CancelFunc){
	return context.WithTimeout(context.Background(), time.Duration(secs)*time.Second)

}
func CreateJwtToken(userID string, username string, email string, role string)(string, error){
	accessTokenExpiry := getEnvDuration("ACCESS_TOKEN_EXPIRY", 15)
	claims := jwt.MapClaims{
		"user_id": userID,
		"username": username,
		"email": email,
		"role": role,
		"exp": time.Now().Add(accessTokenExpiry).Unix(),
		"type": "access",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == ""{
		return "", fmt.Errorf("JWT_SECRET environment variable not set")
	}
	tokenString, err := token.SignedString([]byte(jwtSecret))
	return tokenString, err
}

func CreateRefreshToken(userID string) (string, error) {
	refreshTokenExpiry := getEnvDuration("REFRESH_TOKEN_EXPIRY", 10080) // Default 7 days
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp": time.Now().Add(refreshTokenExpiry).Unix(),
		"type": "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET environment variable not set")
	}
	tokenString, err := token.SignedString([]byte(jwtSecret))
	return tokenString, err
}

func ValidateRefreshToken(tokenString string) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET environment variable not set")
	}
	
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
	
	if err != nil {
		return "", err
	}
	
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}
	
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return "", fmt.Errorf("invalid token type")
	}
	
	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid user_id in token")
	}
	
	return userID, nil
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

func GenerateRandomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func StoreOAuthState(state string) {
	oauthStateExpiry := getEnvDuration("OAUTH_STATE_EXPIRY", 10)
	stateMutex.Lock()
	defer stateMutex.Unlock()
	oauthStates[state] = time.Now().Add(oauthStateExpiry)
	go cleanupExpiredStates()
}

func ValidateOAuthState(state string) bool {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	expiry, exists := oauthStates[state]
	if !exists {
		return false
	}
	
	if time.Now().After(expiry) {
		delete(oauthStates, state)
		return false
	}
	
	delete(oauthStates, state)
	return true
}

func cleanupExpiredStates() {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	
	now := time.Now()
	for state, expiry := range oauthStates {
		if now.After(expiry) {
			delete(oauthStates, state)
		}
	}
}

func GeneratePlanCode(planName string, interval string) string {
	code := ""
	for _, char := range planName {
		if char == ' ' {
			code += "-"
		} else if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			if char >= 'A' && char <= 'Z' {
				code += string(char + 32)
			} else {
				code += string(char)
			}
		}
	}
	
	if interval != "" && interval != "free" {
		code += "-" + interval
	}
	
	timestamp := time.Now().Unix()
	code += fmt.Sprintf("-%d", timestamp%10000)
	
	return code
}

func GenerateTransactionReference(userID string) string {
	timestamp := time.Now().Unix()
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomStr := base64.URLEncoding.EncodeToString(randomBytes)[:8]
	return fmt.Sprintf("TXN-%s-%d-%s", userID[:8], timestamp, randomStr)
}