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
	"golang.org/x/oauth2/facebook"
)

var facebookOAuthConfig = &oauth2.Config{
	ClientID:     os.Getenv("FACEBOOK_APP_ID"),
	ClientSecret: os.Getenv("FACEBOOK_APP_SECRET"),
	RedirectURL:  os.Getenv("FACEBOOK_REDIRECT_URL"),
	Scopes:       []string{"pages_manage_posts", "pages_read_engagement", "public_profile"},
	Endpoint:     facebook.Endpoint,
}

type FacebookPostResponse struct {
	ID string `json:"id"`
}

type FacebookErrorResponse struct {
	Error struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      int    `json:"code"`
		FBTraceID string `json:"fbtrace_id"`
	} `json:"error"`
}

type FacebookPageResponse struct {
	Data []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		AccessToken string `json:"access_token"`
	} `json:"data"`
}

func GetFacebookAuthURL(state string) string {
	return facebookOAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func ExchangeFacebookCodeForToken(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := facebookOAuthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	return token, nil
}

func RefreshFacebookToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{RefreshToken: refreshToken}
	tokenSource := facebookOAuthConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	return newToken, nil
}

func GetFacebookPages(ctx context.Context, accessToken string) ([]struct {
	ID          string
	Name        string
	AccessToken string
}, error) {
	url := "https://graph.facebook.com/v18.0/me/accounts"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	q := req.URL.Query()
	q.Add("access_token", accessToken)
	req.URL.RawQuery = q.Encode()

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

	if resp.StatusCode != http.StatusOK {
		var errResp FacebookErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
			return nil, fmt.Errorf("failed to get pages, status: %s", resp.Status)
		}
		return nil, fmt.Errorf("facebook API error: %s (code: %d)", errResp.Error.Message, errResp.Error.Code)
	}

	var pageResp FacebookPageResponse
	if err := json.Unmarshal(bodyBytes, &pageResp); err != nil {
		return nil, fmt.Errorf("failed to parse pages response: %w", err)
	}

	pages := make([]struct {
		ID          string
		Name        string
		AccessToken string
	}, len(pageResp.Data))

	for i, page := range pageResp.Data {
		pages[i].ID = page.ID
		pages[i].Name = page.Name
		pages[i].AccessToken = page.AccessToken
	}

	return pages, nil
}

func PostToFacebookPage(ctx context.Context, pageAccessToken, pageID, message string) (string, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/feed", pageID)
	
	body := map[string]interface{}{
		"message":      message,
		"access_token": pageAccessToken,
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
		var errResp FacebookErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
			return "", fmt.Errorf("failed to post to Facebook, status: %s, body: %s", resp.Status, string(bodyBytes))
		}
		return "", fmt.Errorf("facebook API error: %s (code: %d)", errResp.Error.Message, errResp.Error.Code)
	}

	var postResp FacebookPostResponse
	if err := json.Unmarshal(bodyBytes, &postResp); err != nil {
		return "", fmt.Errorf("failed to parse post response: %w", err)
	}

	return postResp.ID, nil
}

func PostToFacebookWithRetry(ctx context.Context, token *oauth2.Token, pageID, message string) (string, *oauth2.Token, error) {
	pages, err := GetFacebookPages(ctx, token.AccessToken)
	if err != nil && utils.IsUnauthorizedError(err) {
		newToken, refreshErr := RefreshFacebookToken(ctx, token.RefreshToken)
		if refreshErr != nil {
			return "", token, fmt.Errorf("failed to refresh token: %w", refreshErr)
		}
		
		pages, err = GetFacebookPages(ctx, newToken.AccessToken)
		if err != nil {
			return "", newToken, fmt.Errorf("failed to get pages after refresh: %w", err)
		}
		token = newToken
	} else if err != nil {
		return "", token, fmt.Errorf("failed to get pages: %w", err)
	}

	var pageAccessToken string
	for _, page := range pages {
		if page.ID == pageID {
			pageAccessToken = page.AccessToken
			break
		}
	}
	
	if pageAccessToken == "" && len(pages) > 0 {
		pageAccessToken = pages[0].AccessToken
		pageID = pages[0].ID
	}

	if pageAccessToken == "" {
		return "", token, fmt.Errorf("no pages found")
	}

	postID, err := PostToFacebookPage(ctx, pageAccessToken, pageID, message)
	if err != nil {
		return "", token, fmt.Errorf("failed to post to Facebook: %w", err)
	}

	return postID, token, nil
}
