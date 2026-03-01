package romm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-romm-sync/types"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"go-romm-sync/utils/fileio"
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
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Login authenticates with the RomM server and stores the access token
func (c *Client) Login(username, password string) (string, error) {
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)
	data.Set("scope", "roms.read platforms.read assets.read assets.write")

	req, err := http.NewRequest("POST", c.BaseURL+"/api/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Client.Do(req) //nolint:bodyclose // body is closed via fileio.Close wrapper
	if err != nil {
		return "", fmt.Errorf("failed to perform login request: %w", err)
	}
	defer fileio.Close(resp.Body, nil, "Login: Failed to close response body")

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
//
// types with different JSON decode strategies; a generic refactor would add complexity without benefit.
//
//nolint:dupl // GetLibrary/GetPlatforms have similar pagination structures but operate on different
func (c *Client) GetLibrary(limit, offset int, platformID int) ([]types.Game, int, error) {
	if c.Token == "" {
		return nil, 0, fmt.Errorf("not authenticated")
	}

	// Construct URL with pagination and filtering parameters
	u, err := url.Parse(c.BaseURL + "/api/roms")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse base URL: %w", err)
	}
	q := u.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("offset", fmt.Sprintf("%d", offset))
	if platformID > 0 {
		q.Set("platform_ids", fmt.Sprintf("%d", platformID))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), http.NoBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create library request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req) //nolint:bodyclose // body is closed via fileio.Close
	if err != nil {
		return nil, 0, fmt.Errorf("failed to perform library request: %w", err)
	}
	defer fileio.Close(resp.Body, nil, "GetLibrary: Failed to close response body")

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("library fetch failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Check if response is an array or object (pagination)
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, 0, fmt.Errorf("failed to decode library response: %w", err)
	}

	var pageItems []types.Game
	totalCount := 0

	// Try unmarshalling as array first (backward compatibility or non-paginated)
	if err := json.Unmarshal(raw, &pageItems); err == nil {
		totalCount = len(pageItems)
	} else {
		// Try unmarshalling as paginated object
		var paginated struct {
			Items    []types.Game `json:"items"`
			Total    int          `json:"total_count"`
			TotalAlt int          `json:"total"`
			Total3   int          `json:"count"`
			Total4   int          `json:"total_results"`
		}
		if err := json.Unmarshal(raw, &paginated); err == nil && paginated.Items != nil {
			pageItems = paginated.Items
			totalCount = paginated.Total
			if totalCount == 0 {
				totalCount = paginated.TotalAlt
			}
			if totalCount == 0 {
				totalCount = paginated.Total3
			}
			if totalCount == 0 {
				totalCount = paginated.Total4
			}

			if totalCount == 0 {
				totalCount = len(pageItems)
			}
		} else {
			return nil, 0, fmt.Errorf("unknown library response format: %s", string(raw))
		}
	}

	return pageItems, totalCount, nil
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

	req, err := http.NewRequest("GET", targetURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create cover request: %w", err)
	}

	// Only send authorization if it's an internal RomM request
	if c.shouldSendToken(targetURL) {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.Client.Do(req) //nolint:bodyclose // body is closed via fileio.Close wrapper
	if err != nil {
		return nil, fmt.Errorf("failed to perform cover request: %w", err)
	}
	defer fileio.Close(resp.Body, nil, "DownloadCover: Failed to close response body")

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover fetch failed with status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// GetPlatforms fetches the list of platforms
//
// types with different JSON decode strategies; a generic refactor would add complexity without benefit.
//
//nolint:dupl // GetLibrary/GetPlatforms have similar pagination structures but operate on different
func (c *Client) GetPlatforms(limit, offset int) ([]types.Platform, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	urlStr := fmt.Sprintf("%s/api/platforms?limit=%d&offset=%d", c.BaseURL, limit, offset)
	req, err := http.NewRequest("GET", urlStr, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create platforms request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req) //nolint:bodyclose // body is closed via fileio.Close wrapper
	if err != nil {
		return nil, fmt.Errorf("failed to perform platforms request: %w", err)
	}
	defer fileio.Close(resp.Body, nil, "GetPlatforms: Failed to close response body")

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

	return pageItems, nil
}

// GetRom fetches a single ROM by its ID
func (c *Client) GetRom(id uint) (types.Game, error) {
	if c.Token == "" {
		return types.Game{}, fmt.Errorf("not authenticated")
	}

	urlStr := fmt.Sprintf("%s/api/roms/%d", c.BaseURL, id)
	req, err := http.NewRequest("GET", urlStr, http.NoBody)
	if err != nil {
		return types.Game{}, fmt.Errorf("failed to create ROM request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req) //nolint:bodyclose // body is closed via fileio.Close wrapper
	if err != nil {
		return types.Game{}, fmt.Errorf("failed to perform ROM request: %w", err)
	}
	defer fileio.Close(resp.Body, nil, "GetRom: Failed to close response body")

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
func (c *Client) DownloadFile(game *types.Game) (reader io.ReadCloser, filename string, err error) {
	if c.Token == "" {
		return nil, "", fmt.Errorf("not authenticated")
	}

	filename = filepath.Base(game.FullPath)
	escapedFilename := url.PathEscape(filename)

	urlPath := fmt.Sprintf("%s/api/roms/%d/content/%s", c.BaseURL, game.ID, escapedFilename)
	req, err := http.NewRequest("GET", urlPath, http.NoBody)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create download request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req) //nolint:bodyclose // body is closed via fileio.Close wrapper
	if err != nil {
		return nil, "", fmt.Errorf("failed to perform download request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		fileio.Close(resp.Body, nil, "DownloadFile: Failed to close response body")
		return nil, "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Double check Content-Disposition if the backend assigned an explicit download name
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" && strings.Contains(cd, "filename=") {
		parts := strings.Split(cd, "filename=")
		if len(parts) > 1 {
			filename = strings.Trim(parts[1], "\"")
		}
	}

	return resp.Body, filename, nil
}

// UploadSave uploads a save file to RomM
func (c *Client) UploadSave(romID uint, emulator, filename string, content []byte) error {
	return c.uploadAsset(romID, emulator, filename, content, "saves", "saveFile")
}

// UploadState uploads a save state file to RomM
func (c *Client) UploadState(romID uint, emulator, filename string, content []byte) error {
	return c.uploadAsset(romID, emulator, filename, content, "states", "stateFile")
}

func (c *Client) uploadAsset(romID uint, emulator, filename string, content []byte, endpoint, fieldName string) error {
	if c.Token == "" {
		return fmt.Errorf("not authenticated")
	}

	params := url.Values{}
	params.Set("rom_id", fmt.Sprintf("%d", romID))
	params.Set("emulator", emulator)

	urlStr := fmt.Sprintf("%s/api/%s?%s", c.BaseURL, endpoint, params.Encode())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	_, err = part.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write content to form file: %w", err)
	}
	err = writer.Close()
	if err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", urlStr, body)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("accept", "application/json")

	resp, err := c.Client.Do(req) //nolint:bodyclose // body is closed via fileio.Close wrapper
	if err != nil {
		return fmt.Errorf("failed to perform upload request: %w", err)
	}
	defer fileio.Close(resp.Body, nil, "uploadAsset: Failed to close response body")

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetSaves fetches the list of saves from the RomM server for a given ROM
func (c *Client) GetSaves(romID uint) ([]types.ServerSave, error) {
	return fetchAssets[types.ServerSave](c, fmt.Sprintf("%s/api/saves?rom_id=%d", c.BaseURL, romID), "saves")
}

// GetStates fetches the list of states from the RomM server for a given ROM
func (c *Client) GetStates(romID uint) ([]types.ServerState, error) {
	return fetchAssets[types.ServerState](c, fmt.Sprintf("%s/api/states?rom_id=%d", c.BaseURL, romID), "states")
}

// fetchAssets is a generic helper that fetches a JSON list from a RomM API endpoint.
func fetchAssets[T any](c *Client, urlStr, assetType string) ([]T, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	req, err := http.NewRequest("GET", urlStr, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s request: %w", assetType, err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req) //nolint:bodyclose // body is closed via fileio.Close wrapper
	if err != nil {
		return nil, fmt.Errorf("failed to perform %s request: %w", assetType, err)
	}
	defer fileio.Close(resp.Body, nil, "fetchAssets: Failed to close response body")

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s fetch failed with status %d: %s", assetType, resp.StatusCode, string(body))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)

	if len(bodyBytes) == 0 {
		return []T{}, nil
	}

	var items []T
	if err := json.Unmarshal(bodyBytes, &items); err != nil {
		return nil, fmt.Errorf("failed to decode %s response: %w", assetType, err)
	}

	return items, nil
}

// DownloadSave fetches a save file from RomM
func (c *Client) DownloadSave(filePath string) (reader io.ReadCloser, filename string, err error) {
	return c.downloadAsset(filePath, "unknown.sav")
}

// DownloadState fetches a state file from RomM
func (c *Client) DownloadState(filePath string) (reader io.ReadCloser, filename string, err error) {
	return c.downloadAsset(filePath, "unknown.state")
}

func (c *Client) downloadAsset(filePath, fallbackFilename string) (reader io.ReadCloser, filename string, err error) {
	if c.Token == "" {
		return nil, "", fmt.Errorf("not authenticated")
	}

	urlPath := fmt.Sprintf("%s/api/raw/assets/%s", c.BaseURL, strings.TrimPrefix(filePath, "/"))
	req, err := http.NewRequest("GET", urlPath, http.NoBody)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create download request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to perform download request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fileio.Close(resp.Body, nil, "downloadAsset: Failed to close response body")
		return nil, "", fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	filename = fallbackFilename
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" && strings.Contains(cd, "filename=") {
		parts := strings.Split(cd, "filename=")
		if len(parts) > 1 {
			filename = strings.Trim(parts[1], "\"")
		}
	}

	return resp.Body, filename, nil
}

// shouldSendToken determines if the authentication token should be sent to the target URL.
// It returns true if the target URL is relative or if it matches the BaseURL's scheme and host.
func (c *Client) shouldSendToken(targetURL string) bool {
	if !strings.HasPrefix(targetURL, "http") {
		return true // Relative URL
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		return false
	}

	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return false
	}

	// Compare scheme and host (which includes port)
	return target.Scheme == base.Scheme && target.Host == base.Host
}
