package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type TwitterService struct {
	bearerToken string
	client      *http.Client
}

type twitterUserResponse struct {
	Data struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type twitterTweetsResponse struct {
	Data []struct {
		ID        string `json:"id"`
		Text      string `json:"text"`
		CreatedAt string `json:"created_at"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func NewTwitterService(bearerToken string) *TwitterService {
	return &TwitterService{
		bearerToken: bearerToken,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

// FetchUserTimeline fetches recent tweets for a given username and returns them as Headlines.
func (t *TwitterService) FetchUserTimeline(username string) ([]Headline, error) {
	// Step 1: resolve username → user ID
	userURL := fmt.Sprintf("https://api.twitter.com/2/users/by/username/%s", username)
	userResp, err := t.doRequest(userURL)
	if err != nil {
		return nil, fmt.Errorf("twitter user lookup failed: %w", err)
	}

	var userData twitterUserResponse
	if err := json.Unmarshal(userResp, &userData); err != nil {
		return nil, fmt.Errorf("failed to parse twitter user response: %w", err)
	}
	if len(userData.Errors) > 0 {
		return nil, fmt.Errorf("twitter API error: %s", userData.Errors[0].Message)
	}
	if userData.Data.ID == "" {
		return nil, fmt.Errorf("twitter user not found: %s", username)
	}

	// Step 2: fetch timeline
	tweetsURL := fmt.Sprintf(
		"https://api.twitter.com/2/users/%s/tweets?max_results=10&tweet.fields=created_at&exclude=retweets,replies",
		userData.Data.ID,
	)
	tweetsResp, err := t.doRequest(tweetsURL)
	if err != nil {
		return nil, fmt.Errorf("twitter timeline fetch failed: %w", err)
	}

	var tweetsData twitterTweetsResponse
	if err := json.Unmarshal(tweetsResp, &tweetsData); err != nil {
		return nil, fmt.Errorf("failed to parse twitter tweets response: %w", err)
	}
	if len(tweetsData.Errors) > 0 {
		return nil, fmt.Errorf("twitter API error: %s", tweetsData.Errors[0].Message)
	}

	headlines := make([]Headline, 0, len(tweetsData.Data))
	for _, tweet := range tweetsData.Data {
		publishedAt := time.Now()
		if tweet.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, tweet.CreatedAt); err == nil {
				publishedAt = t
			}
		}

		// Use first line as title, full text as description
		title := tweet.Text
		if idx := strings.Index(title, "\n"); idx > 0 {
			title = title[:idx]
		}
		if len(title) > 120 {
			title = title[:120] + "..."
		}

		headlines = append(headlines, Headline{
			ID:          tweet.ID,
			Title:       title,
			Description: tweet.Text,
			URL:         fmt.Sprintf("https://x.com/%s/status/%s", username, tweet.ID),
			Source:      "@" + userData.Data.Username,
			Category:    "Social",
			PublishedAt: publishedAt,
		})
	}

	logrus.WithFields(logrus.Fields{
		"username": username,
		"count":    len(headlines),
	}).Info("Fetched Twitter timeline")

	return headlines, nil
}

func (t *TwitterService) doRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.bearerToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitter API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// NewTwitterServiceFromEnv creates a TwitterService from the TWITTER_BEARER_TOKEN env var.
func NewTwitterServiceFromEnv() *TwitterService {
	token := os.Getenv("TWITTER_BEARER_TOKEN")
	if token == "" {
		return nil
	}
	return NewTwitterService(token)
}
