package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	PaystackBaseURL = "https://api.paystack.co"
)

type PaystackService struct {
	secretKey string
	client    *http.Client
}

var paystackService *PaystackService

type PaystackInitializeResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AuthorizationURL string `json:"authorization_url"`
		AccessCode       string `json:"access_code"`
		Reference        string `json:"reference"`
	} `json:"data"`
}

type PaystackVerifyResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID              int64  `json:"id"`
		Domain          string `json:"domain"`
		Status          string `json:"status"`
		Reference       string `json:"reference"`
		Amount          int    `json:"amount"`
		Message         string `json:"message"`
		GatewayResponse string `json:"gateway_response"`
		PaidAt          string `json:"paid_at"`
		CreatedAt       string `json:"created_at"`
		Channel         string `json:"channel"`
		Currency        string `json:"currency"`
		IPAddress       string `json:"ip_address"`
		Customer        struct {
			ID           int    `json:"id"`
			Email        string `json:"email"`
			CustomerCode string `json:"customer_code"`
		} `json:"customer"`
	} `json:"data"`
}

func InitPaystackService() error {
	secretKey := os.Getenv("PAYSTACK_SECRET_KEY")
	if secretKey == "" {
		return fmt.Errorf("PAYSTACK_SECRET_KEY not found in environment variables")
	}

	paystackService = &PaystackService{
		secretKey: secretKey,
		client:    &http.Client{},
	}

	return nil
}

func InitializeTransaction(email string, amount int, reference string, callbackURL string) (*PaystackInitializeResponse, error) {
	if paystackService == nil {
		return nil, fmt.Errorf("paystack service not initialized")
	}

	payload := map[string]interface{}{
		"email":     email,
		"amount":    amount, 
		"reference": reference,
	}

	if callbackURL != "" {
		payload["callback_url"] = callbackURL
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", PaystackBaseURL+"/transaction/initialize", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+paystackService.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := paystackService.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var initResponse PaystackInitializeResponse
	if err := json.Unmarshal(body, &initResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if !initResponse.Status {
		return nil, fmt.Errorf("paystack error: %s", initResponse.Message)
	}

	return &initResponse, nil
}

func VerifyTransaction(reference string) (*PaystackVerifyResponse, error) {
	if paystackService == nil {
		return nil, fmt.Errorf("paystack service not initialized")
	}

	req, err := http.NewRequest("GET", PaystackBaseURL+"/transaction/verify/"+reference, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+paystackService.secretKey)

	resp, err := paystackService.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var verifyResponse PaystackVerifyResponse
	if err := json.Unmarshal(body, &verifyResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if !verifyResponse.Status {
		return nil, fmt.Errorf("paystack error: %s", verifyResponse.Message)
	}

	return &verifyResponse, nil
}

func GenerateTransactionReference(userID string) string {
	return fmt.Sprintf("TXN-%s-%d", userID, time.Now().Unix())
}

func TimeNow() interface {
	Unix() int64
} {
	return timeNow{}
}

type timeNow struct{}

func (timeNow) Unix() int64 {
	return 0 
}
