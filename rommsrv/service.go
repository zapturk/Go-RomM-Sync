package rommsrv

import (
	"encoding/base64"
	"fmt"
	"go-romm-sync/constants"
	"go-romm-sync/retroarch"
	"go-romm-sync/romm"
	"go-romm-sync/types"
	"os"
	"path/filepath"
	"strings"
)

// ConfigProvider defines the configuration needed for RomM services.
type ConfigProvider interface {
	GetRomMHost() string
	GetUsername() string
	GetPassword() string
}

// Service handles interactions with the RomM server and manages local caches for assets.
type Service struct {
	config ConfigProvider
	client *romm.Client
}

// New creates a new RomM service.
func New(cfg ConfigProvider) *Service {
	host := cfg.GetRomMHost()
	return &Service{
		config: cfg,
		client: romm.NewClient(host),
	}
}

// Login authenticates with the RomM server and returns a token.
func (s *Service) Login() (string, error) {
	host := s.config.GetRomMHost()
	user := s.config.GetUsername()
	pass := s.config.GetPassword()

	if host == "" || user == "" || pass == "" {
		return "", fmt.Errorf("missing configuration: host, username, or password")
	}

	// Ensure client is up to date with current config
	if s.client.BaseURL == "" || s.client.BaseURL != host {
		s.client = romm.NewClient(host)
	}

	token, err := s.client.Login(user, pass)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) GetClient() *romm.Client {
	return s.client
}

// GetLibrary fetches a page of the game library from RomM, optionally filtered by platform.
func (s *Service) GetLibrary(limit, offset, platformID int) ([]types.Game, int, error) {
	return s.client.GetLibrary(limit, offset, platformID)
}

// GetPlatforms fetches a page of supported platforms from RomM.
// It filters out platforms that aren't recognized by RetroArch mappings.
func (s *Service) GetPlatforms(limit, offset int) ([]types.Platform, int, error) {
	const batchSize = 100
	const maxScan = 2000
	var supported []types.Platform

	currentOffset := 0
	foundCount := 0

	for {
		batch, totalOnServer, err := s.client.GetPlatforms(batchSize, currentOffset)
		if err != nil {
			return nil, 0, err
		}
		if len(batch) == 0 {
			break
		}

		// 1. Collect platforms for the current page
		for _, p := range batch {
			if isPlatformSupported(p) {
				if foundCount >= offset && len(supported) < limit {
					supported = append(supported, p)
				}
				foundCount++
			}
		}

		currentOffset += len(batch)

		// 2. Optimization: if we've filled our page OR reached an upper scan limit
		if (len(supported) >= limit && currentOffset >= totalOnServer) || currentOffset >= maxScan {
			// Scan remaining server platforms only to get an accurate total count
			if currentOffset < totalOnServer && currentOffset < maxScan {
				foundCount += s.countRemainingSupported(totalOnServer, currentOffset, batchSize, maxScan)
			}
			break
		}

		if currentOffset >= totalOnServer {
			break
		}
	}

	return supported, foundCount, nil
}

// countRemainingSupported continues scanning platforms from the server just to update the supported count.
func (s *Service) countRemainingSupported(totalOnServer, startOffset, batchSize, maxScan int) int {
	additionalCount := 0
	currentOffset := startOffset
	for currentOffset < totalOnServer && currentOffset < maxScan {
		batch, _, err := s.client.GetPlatforms(batchSize, currentOffset)
		if err != nil || len(batch) == 0 {
			break
		}
		for _, p := range batch {
			if isPlatformSupported(p) {
				additionalCount++
			}
		}
		currentOffset += len(batch)
	}
	return additionalCount
}

func isPlatformSupported(p types.Platform) bool {
	// Check if supported by RetroArch and has games
	return p.RomCount > 0 && (retroarch.IdentifyPlatform(p.Name) != "" || retroarch.IdentifyPlatform(p.Slug) != "")
}

// GetRom fetches a single ROM from RomM.
func (s *Service) GetRom(id uint) (types.Game, error) {
	return s.client.GetRom(id)
}

// GetCover returns the base64 encoded cover image for a game, using a local cache.
func (s *Service) GetCover(romID uint, coverURL string) (string, error) {
	if coverURL == "" {
		return "", nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home dir: %w", err)
	}
	cacheDir := filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.CoversDir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create cache dir: %w", err)
	}

	ext := filepath.Ext(coverURL)
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("%d%s", romID, ext)
	cachePath := filepath.Join(cacheDir, filename)

	var data []byte
	if _, err := os.Stat(cachePath); err == nil {
		data, err = os.ReadFile(cachePath)
		if err != nil {
			return "", fmt.Errorf("failed to read cached cover: %w", err)
		}
	} else {
		data, err = s.client.DownloadCover(coverURL)
		if err != nil {
			return "", fmt.Errorf("failed to download cover: %w", err)
		}

		_ = os.WriteFile(cachePath, data, 0o644)
	}

	return toDataURI(data, ext), nil
}

// GetPlatformCover returns the data URI for the platform cover, using a local cache.
func (s *Service) GetPlatformCover(platformID uint, slug string) (string, error) {
	if slug == "" {
		return "", nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home dir: %w", err)
	}
	cacheDir := filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.PlatformsDir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create cache dir: %w", err)
	}

	extensions := []string{".svg", ".ico", ".png", ".jpg"}

	// 1. Try Cache
	data, foundExt := s.tryGetPlatformCoverFromCache(cacheDir, platformID, extensions)

	// 2. Try Download if not in cache
	if data == nil {
		data, foundExt = s.tryDownloadPlatformCover(slug, extensions)
		if data == nil {
			return "", fmt.Errorf("failed to download cover")
		}

		// Save to cache
		filename := fmt.Sprintf("%d%s", platformID, foundExt)
		cachePath := filepath.Join(cacheDir, filename)
		_ = os.WriteFile(cachePath, data, 0o644)
	}

	return toDataURI(data, foundExt), nil
}

func (s *Service) tryGetPlatformCoverFromCache(cacheDir string, platformID uint, extensions []string) (data []byte, ext string) {
	for _, ext := range extensions {
		filename := fmt.Sprintf("%d%s", platformID, ext)
		cachePath := filepath.Join(cacheDir, filename)
		if _, err := os.Stat(cachePath); err == nil {
			if d, err := os.ReadFile(cachePath); err == nil {
				return d, ext
			}
		}
	}
	return nil, ""
}

func (s *Service) tryDownloadPlatformCover(slug string, extensions []string) (data []byte, ext string) {
	for _, ext := range extensions {
		url := fmt.Sprintf("/assets/platforms/%s%s", slug, ext)
		if d, err := s.client.DownloadCover(url); err == nil {
			return d, ext
		}
	}

	if strings.Contains(slug, "-") {
		altSlug := strings.ReplaceAll(slug, "-", "_")
		for _, ext := range extensions {
			url := fmt.Sprintf("/assets/platforms/%s%s", altSlug, ext)
			if d, err := s.client.DownloadCover(url); err == nil {
				return d, ext
			}
		}
	}
	return nil, ""
}

func getMimeType(ext string) string {
	switch ext {
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}

func toDataURI(data []byte, ext string) string {
	mimeType := getMimeType(ext)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
}

// GetServerSaves gets a list of server saves from RomM.
func (s *Service) GetServerSaves(id uint) ([]types.ServerSave, error) {
	return s.client.GetSaves(id)
}

// GetServerStates gets a list of server states from RomM.
func (s *Service) GetServerStates(id uint) ([]types.ServerState, error) {
	return s.client.GetStates(id)
}
