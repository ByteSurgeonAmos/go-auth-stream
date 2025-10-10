package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/ByteSurgeonAmos/go-auth-stream/utils"
	"golang.org/x/oauth2"
)

var instagramOAuthConfig = &oauth2.Config{
	ClientID:     os.Getenv("INSTAGRAM_CLIENT_ID"),
	ClientSecret: os.Getenv("INSTAGRAM_CLIENT_SECRET"),
	RedirectURL:  os.Getenv("INSTAGRAM_REDIRECT_URL"),
	Scopes:       []string{"instagram_basic", "instagram_content_publish"},
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://api.instagram.com/oauth/authorize",
		TokenURL: "https://api.instagram.com/oauth/access_token",
	},
}

type InstagramMediaResponse struct {
	ID string `json:"id"`
}

type InstagramErrorResponse struct {
	Error struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		FBTraceID    string `json:"fbtrace_id"`
	} `json:"error"`
}

type InstagramAccountResponse struct {
	InstagramBusinessAccount struct {
		ID string `json:"id"`
	} `json:"instagram_business_account"`
	ID string `json:"id"`
}

func GetInstagramAuthURL(state string) string {
	return instagramOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func ExchangeInstagramCodeForToken(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := instagramOAuthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	
	longLivedToken, err := exchangeForLongLivedToken(ctx, token.AccessToken)
	if err != nil {
		return token, nil 
	}
	
	return longLivedToken, nil
}

func exchangeForLongLivedToken(ctx context.Context, shortLivedToken string) (*oauth2.Token, error) {
	url := fmt.Sprintf("https://graph.instagram.com/access_token?grant_type=ig_exchange_token&client_secret=%s&access_token=%s",
		os.Getenv("INSTAGRAM_CLIENT_SECRET"), shortLivedToken)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &oauth2.Token{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenResp.TokenType,
		Expiry:      time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}

func RefreshInstagramToken(ctx context.Context, accessToken string) (*oauth2.Token, error) {
	url := fmt.Sprintf("https://graph.instagram.com/refresh_access_token?grant_type=ig_refresh_token&access_token=%s", accessToken)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &oauth2.Token{
		AccessToken: tokenResp.AccessToken,
		TokenType:   tokenResp.TokenType,
		Expiry:      time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}

func GetInstagramBusinessAccountID(ctx context.Context, accessToken, pageID string) (string, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s?fields=instagram_business_account&access_token=%s", pageID, accessToken)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var accountResp InstagramAccountResponse
	if err := json.Unmarshal(bodyBytes, &accountResp); err != nil {
		return "", fmt.Errorf("failed to parse account response: %w", err)
	}

	return accountResp.InstagramBusinessAccount.ID, nil
}

func CreateInstagramMediaContainer(ctx context.Context, accessToken, igAccountID, caption, imageURL string) (string, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/media", igAccountID)
	
	body := map[string]interface{}{
		"image_url":    imageURL,
		"caption":      caption,
		"access_token": accessToken,
	}
	
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp InstagramErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
			return "", fmt.Errorf("failed to create media container, status: %s, body: %s", resp.Status, string(bodyBytes))
		}
		return "", fmt.Errorf("instagram API error: %s (code: %d)", errResp.Error.Message, errResp.Error.Code)
	}

	var mediaResp InstagramMediaResponse
	if err := json.Unmarshal(bodyBytes, &mediaResp); err != nil {
		return "", fmt.Errorf("failed to parse media response: %w", err)
	}

	return mediaResp.ID, nil
}

func PublishInstagramMedia(ctx context.Context, accessToken, igAccountID, creationID string) (string, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/media_publish", igAccountID)
	
	body := map[string]interface{}{
		"creation_id":  creationID,
		"access_token": accessToken,
	}
	
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp InstagramErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
			return "", fmt.Errorf("failed to publish media, status: %s, body: %s", resp.Status, string(bodyBytes))
		}
		return "", fmt.Errorf("instagram API error: %s (code: %d)", errResp.Error.Message, errResp.Error.Code)
	}

	var publishResp InstagramMediaResponse
	if err := json.Unmarshal(bodyBytes, &publishResp); err != nil {
		return "", fmt.Errorf("failed to parse publish response: %w", err)
	}

	return publishResp.ID, nil
}

func PostToInstagram(ctx context.Context, accessToken, igAccountID, caption, imageURL string) (string, error) {
	creationID, err := CreateInstagramMediaContainer(ctx, accessToken, igAccountID, caption, imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to create media container: %w", err)
	}

	postID, err := PublishInstagramMedia(ctx, accessToken, igAccountID, creationID)
	if err != nil {
		return "", fmt.Errorf("failed to publish media: %w", err)
	}

	return postID, nil
}

func PostToInstagramWithRetry(ctx context.Context, token *oauth2.Token, igAccountID, caption, imageURL string) (string, *oauth2.Token, error) {
	postID, err := PostToInstagram(ctx, token.AccessToken, igAccountID, caption, imageURL)
	
	if err != nil && utils.IsUnauthorizedError(err) {
		newToken, refreshErr := RefreshInstagramToken(ctx, token.AccessToken)
		if refreshErr != nil {
			return "", token, fmt.Errorf("failed to refresh token: %w", refreshErr)
		}

		postID, retryErr := PostToInstagram(ctx, newToken.AccessToken, igAccountID, caption, imageURL)
		if retryErr != nil {
			return "", newToken, fmt.Errorf("failed to post to Instagram after refresh: %w", retryErr)
		}

		return postID, newToken, nil
	}

	return postID, token, err
}
