package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
    "github.com/ByteSurgeonAmos/go-auth-stream/utils"
	"golang.org/x/oauth2"
)

var twitterOAuthConfig = &oauth2.Config{
	ClientID:     os.Getenv("TWITTER_CLIENT_ID"),
	ClientSecret: os.Getenv("TWITTER_CLIENT_SECRET"),
	RedirectURL:  os.Getenv("OAUTH_REDIRECT_URL"),
	Scopes:       []string{"tweet.read", "tweet.write", "users.read", "offline.access"},
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://twitter.com/i/oauth2/authorize",
		TokenURL: "https://api.twitter.com/2/oauth2/token",
	},
}

type TweetResponse struct {
	Data struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"data"`
}

type ErrorResponse struct {
	Detail string `json:"detail"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	Status int    `json:"status"`
}

type RateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

func GetTwitterAuthURL(state string) string {
	return twitterOAuthConfig.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func ExchangeCodeForToken(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := twitterOAuthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}
	return token, nil
}

func RefreshTwitterToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{RefreshToken: refreshToken}
	tokenSource := twitterOAuthConfig.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	return newToken, nil
}

func PostTweet(ctx context.Context, accessToken, tweetText string) (string, *RateLimitInfo, error) {
	url := "https://api.twitter.com/2/tweets"
	body := map[string]interface{}{
		"text": tweetText,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	rateLimitInfo := parseRateLimitHeaders(resp.Header)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", rateLimitInfo, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
			return "", rateLimitInfo, fmt.Errorf("failed to post tweet, status: %s, body: %s", resp.Status, string(bodyBytes))
		}
		return "", rateLimitInfo, fmt.Errorf("twitter API error: %s - %s (status: %d)", errResp.Title, errResp.Detail, errResp.Status)
	}

	var tweetResp TweetResponse
	if err := json.Unmarshal(bodyBytes, &tweetResp); err != nil {
		return "", rateLimitInfo, fmt.Errorf("failed to parse tweet response: %w", err)
	}

	return tweetResp.Data.ID, rateLimitInfo, nil
}

func parseRateLimitHeaders(headers http.Header) *RateLimitInfo {
	info := &RateLimitInfo{}

	if limit := headers.Get("x-rate-limit-limit"); limit != "" {
		if val, err := strconv.Atoi(limit); err == nil {
			info.Limit = val
		}
	}

	if remaining := headers.Get("x-rate-limit-remaining"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil {
			info.Remaining = val
		}
	}

	if reset := headers.Get("x-rate-limit-reset"); reset != "" {
		if val, err := strconv.ParseInt(reset, 10, 64); err == nil {
			info.Reset = time.Unix(val, 0)
		}
	}

	return info
}

func PostTweetWithRetry(ctx context.Context, token *oauth2.Token, tweetText string) (string, *oauth2.Token, error) {
	tweetID, rateLimitInfo, err := PostTweet(ctx, token.AccessToken, tweetText)
	
	if rateLimitInfo != nil && rateLimitInfo.Remaining == 0 {
		waitDuration := time.Until(rateLimitInfo.Reset)
		return "", token, fmt.Errorf("rate limit exceeded, reset at %s (wait %v)", rateLimitInfo.Reset.Format(time.RFC3339), waitDuration)
	}

	if err != nil && utils.IsUnauthorizedError(err) {
		newToken, refreshErr := RefreshTwitterToken(ctx, token.RefreshToken)
		if refreshErr != nil {
			return "", token, fmt.Errorf("failed to refresh token: %w", refreshErr)
		}

		tweetID, _, retryErr := PostTweet(ctx, newToken.AccessToken, tweetText)
		if retryErr != nil {
			return "", newToken, fmt.Errorf("failed to post tweet after refresh: %w", retryErr)
		}

		return tweetID, newToken, nil
	}

	return tweetID, token, err
}

