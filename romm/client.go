package romm

import (
	"encoding/json"
	"fmt"
	"go-romm-sync/types"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Client handles communication with the RomM API
type Client struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

// NewClient creates a new RomM API client
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{},
	}
}

// Login authenticates with the RomM server and stores the access token
func (c *Client) Login(username, password string) (string, error) {
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)
	data.Set("scope", "roms.read")

	req, err := http.NewRequest("POST", c.BaseURL+"/api/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}

	c.Token = result.AccessToken
	return c.Token, nil
}

// GetLibrary fetches the list of games (ROMs) from the library
func (c *Client) GetLibrary() ([]types.Game, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	req, err := http.NewRequest("GET", c.BaseURL+"/api/roms", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create library request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform library request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("library fetch failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Check if response is an array or object (pagination)
	// We'll decode into a raw message first to check
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode library response: %w", err)
	}

	// Try unmarshalling as array first (backward compatibility)
	var games []types.Game
	if err := json.Unmarshal(raw, &games); err == nil {
		return games, nil
	}

	// Try unmarshalling as paginated object
	var paginated struct {
		Items []types.Game `json:"items"`
	}
	if err := json.Unmarshal(raw, &paginated); err == nil {
		return paginated.Items, nil
	}

	return nil, fmt.Errorf("failed to parse library response: unknown format")
}
