package assets

import (
	"encoding/base64"
	"fmt"
	"go-romm-sync/constants"
	"os"
	"path/filepath"
	"strings"
)

// ConfigProvider defines the configuration needed for asset management.
type ConfigProvider interface {
	GetRomMHost() string
}

// RomMClient defines the interface for downloading covers from RomM.
type RomMClient interface {
	DownloadCover(url string) ([]byte, error)
}

// UIProvider defines logging functionality.
type UIProvider interface {
	LogErrorf(format string, args ...interface{})
}

// Service manages asset caching and conversion.
type Service struct {
	config ConfigProvider
	client RomMClient
	ui     UIProvider
}

// New creates a new Assets service.
func New(cfg ConfigProvider, client RomMClient, ui UIProvider) *Service {
	return &Service{
		config: cfg,
		client: client,
		ui:     ui,
	}
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

// ClearCache removes all cached images.
func (s *Service) ClearCache() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	cacheDirs := []string{
		filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.CoversDir),
		filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.PlatformsDir),
	}

	for _, dir := range cacheDirs {
		if err := os.RemoveAll(dir); err != nil {
			s.ui.LogErrorf("Failed to clear cache directory %s: %v", dir, err)
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			s.ui.LogErrorf("Failed to recreate cache directory %s: %v", dir, err)
		}
	}

	return nil
}
