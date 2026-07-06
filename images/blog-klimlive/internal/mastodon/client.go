package mastodon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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

type mastodonStatus struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

type account struct {
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

	var result mastodonStatus
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.ID, nil
}

func (c *Client) FindStatusByURL(postURL string) (string, error) {
	acct, err := c.verifyCredentials()
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s/api/v1/accounts/%s/statuses?limit=40", c.instanceURL, acct.ID)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch statuses: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("mastodon API returned %d: %s", resp.StatusCode, body)
	}

	var statuses []mastodonStatus
	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		return "", fmt.Errorf("decode statuses: %w", err)
	}

	for _, s := range statuses {
		if strings.Contains(s.Content, postURL) {
			return s.ID, nil
		}
	}

	return "", nil
}

func (c *Client) verifyCredentials() (*account, error) {
	req, err := http.NewRequest("GET", c.instanceURL+"/api/v1/accounts/verify_credentials", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("verify credentials: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mastodon API returned %d: %s", resp.StatusCode, body)
	}

	var acct account
	if err := json.NewDecoder(resp.Body).Decode(&acct); err != nil {
		return nil, fmt.Errorf("decode account: %w", err)
	}

	return &acct, nil
}
