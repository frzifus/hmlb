package mastodon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	instanceURL string
	accessToken string
	httpClient  *http.Client
}

func NewClient(instanceURL, accessToken string) *Client {
	return &Client{
		instanceURL: instanceURL,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type postStatusResponse struct {
	ID string `json:"id"`
}

func (c *Client) PostStatus(status, visibility string) (string, error) {
	form := url.Values{}
	form.Set("status", status)
	form.Set("visibility", visibility)

	req, err := http.NewRequest("POST", c.instanceURL+"/api/v1/statuses", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("post status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("mastodon API returned %d: %s", resp.StatusCode, body)
	}

	var result postStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.ID, nil
}
