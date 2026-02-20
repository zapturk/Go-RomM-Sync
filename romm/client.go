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
	data.Set("scope", "roms.read platforms.read")

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

	var allGames []types.Game
	seenIDs := make(map[uint]bool)
	limit := 100
	offset := 0

	for {
		// Construct URL with pagination parameters
		url := fmt.Sprintf("%s/api/roms?limit=%d&offset=%d", c.BaseURL, limit, offset)
		req, err := http.NewRequest("GET", url, nil)
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

		var pageItems []types.Game

		// Try unmarshalling as array first (backward compatibility or non-paginated)
		if err := json.Unmarshal(raw, &pageItems); err != nil {
			// Try unmarshalling as paginated object
			var paginated struct {
				Items []types.Game `json:"items"`
				Total int          `json:"total_count"` // Guessing field name, catching just items for now
			}
			if err := json.Unmarshal(raw, &paginated); err == nil {
				pageItems = paginated.Items
			} else {
				// Failed both
				return nil, fmt.Errorf("failed to parse library response: unknown format")
			}
		}

		if len(pageItems) == 0 {
			break
		}

		// Check for duplicates (indicates ignored pagination params or end of list loop)
		newItems := false
		for _, game := range pageItems {
			if !seenIDs[game.ID] {
				seenIDs[game.ID] = true
				allGames = append(allGames, game)
				newItems = true
			}
		}

		if !newItems {
			// If all items in this page were already seen, stop to avoid infinite loop
			break
		}

		if len(pageItems) < limit {
			break
		}

		offset += limit
	}

	return allGames, nil
}

// DownloadCover fetches the cover image from the provided URL
func (c *Client) DownloadCover(coverURL string) ([]byte, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	// Handles full URL or relative path
	targetURL := coverURL
	if !strings.HasPrefix(coverURL, "http") {
		targetURL = c.BaseURL + coverURL
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cover request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform cover request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover fetch failed with status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// GetPlatforms fetches the list of platforms
func (c *Client) GetPlatforms() ([]types.Platform, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	var allPlatforms []types.Platform
	seenIDs := make(map[uint]bool)
	limit := 100
	offset := 0

	for {
		url := fmt.Sprintf("%s/api/platforms?limit=%d&offset=%d", c.BaseURL, limit, offset)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create platforms request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.Token)

		resp, err := c.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to perform platforms request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("platforms fetch failed with status %d: %s", resp.StatusCode, string(body))
		}

		var raw json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return nil, fmt.Errorf("failed to decode platforms response: %w", err)
		}

		var pageItems []types.Platform

		if err := json.Unmarshal(raw, &pageItems); err != nil {
			var paginated struct {
				Items []types.Platform `json:"items"`
				Total int              `json:"total_count"`
			}
			if err := json.Unmarshal(raw, &paginated); err == nil {
				pageItems = paginated.Items
			} else {
				return nil, fmt.Errorf("failed to parse platforms response: unknown format")
			}
		}

		if len(pageItems) == 0 {
			break
		}

		newItems := false
		for _, item := range pageItems {
			if !seenIDs[item.ID] {
				seenIDs[item.ID] = true
				allPlatforms = append(allPlatforms, item)
				newItems = true
			}
		}

		if !newItems {
			break
		}

		if len(pageItems) < limit {
			break
		}

		offset += limit
	}

	return allPlatforms, nil
}

// GetRom fetches a single ROM by its ID
func (c *Client) GetRom(id uint) (types.Game, error) {
	if c.Token == "" {
		return types.Game{}, fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf("%s/api/roms/%d", c.BaseURL, id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return types.Game{}, fmt.Errorf("failed to create ROM request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req)
	if err != nil {
		return types.Game{}, fmt.Errorf("failed to perform ROM request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return types.Game{}, fmt.Errorf("ROM fetch failed with status %d: %s", resp.StatusCode, string(body))
	}

	var game types.Game
	if err := json.NewDecoder(resp.Body).Decode(&game); err != nil {
		return types.Game{}, fmt.Errorf("failed to decode ROM response: %w", err)
	}

	return game, nil
}

// DownloadFile fetches a file from RomM and returns a reader and the filename
func (c *Client) DownloadFile(romID uint) (io.ReadCloser, string, error) {
	if c.Token == "" {
		return nil, "", fmt.Errorf("not authenticated")
	}

	url := fmt.Sprintf("%s/api/roms/download?rom_ids=%d", c.BaseURL, romID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create download request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to perform download request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Try to get filename from Content-Disposition
	filename := ""
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		if strings.Contains(cd, "filename=") {
			parts := strings.Split(cd, "filename=")
			if len(parts) > 1 {
				filename = strings.Trim(parts[1], "\"")
			}
		}
	}

	return resp.Body, filename, nil
}
