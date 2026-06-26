package assets

import (
	"encoding/base64"
	"fmt"
	"go-romm-sync/constants"
	"net/http"
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

	extensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"}
	urlExt := strings.ToLower(filepath.Ext(coverURL))
	if urlExt != "" {
		// Prioritize the extension from the cover URL
		extensions = append([]string{urlExt}, extensions...)
	}

	var data []byte
	var foundExt string

	for _, ext := range extensions {
		p := filepath.Join(cacheDir, fmt.Sprintf("%d%s", romID, ext))
		if _, err := os.Stat(p); err == nil {
			if d, err := os.ReadFile(p); err == nil {
				data = d
				foundExt = ext
				break
			}
		}
	}

	if data == nil {
		var err error
		data, err = s.client.DownloadCover(coverURL)
		if err != nil {
			return "", fmt.Errorf("failed to download cover: %w", err)
		}

		ext := urlExt
		if ext == "" {
			ext = ".jpg"
		}
		cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d%s", romID, ext))
		_ = os.WriteFile(cachePath, data, 0o644)
		foundExt = ext
	}

	return toDataURI(data, foundExt), nil
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

// ServeHTTP implements http.Handler to serve cached game covers and platform icons directly, proxying downloads if not cached.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	println("[ServeHTTP] Request received for path:", path, "query:", r.URL.RawQuery)
	if !strings.HasPrefix(path, "/cache/") {
		println("[ServeHTTP] Prefix /cache/ not matched, returning 404")
		http.NotFound(w, r)
		return
	}

	// Determine cache directory and target ID based on path
	var cacheSubdir string
	switch {
	case strings.HasPrefix(path, "/cache/covers/"):
		cacheSubdir = constants.CoversDir
		path = strings.TrimPrefix(path, "/cache/covers/")
	case strings.HasPrefix(path, "/cache/platforms/"):
		cacheSubdir = constants.PlatformsDir
		path = strings.TrimPrefix(path, "/cache/platforms/")
	default:
		println("[ServeHTTP] Subdir covers/platforms not matched, returning 404")
		http.NotFound(w, r)
		return
	}

	// Remove any extension from path to get the ID string
	idStr := path
	if idx := strings.Index(idStr, "."); idx != -1 {
		idStr = idStr[:idx]
	}

	if idStr == "" {
		println("[ServeHTTP] Empty ID, returning 404")
		http.NotFound(w, r)
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Home dir not found", http.StatusInternalServerError)
		return
	}
	cacheDir := filepath.Join(homeDir, constants.AppDir, constants.CacheDir, cacheSubdir)

	// Look in local cache first
	var data []byte
	var foundExt string
	extensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico"}

	coverURL := r.URL.Query().Get("url")
	urlExt := strings.ToLower(filepath.Ext(coverURL))
	if urlExt != "" {
		extensions = append([]string{urlExt}, extensions...)
	}

	for _, ext := range extensions {
		p := filepath.Join(cacheDir, idStr+ext)
		if _, err := os.Stat(p); err == nil {
			if d, err := os.ReadFile(p); err == nil {
				data = d
				foundExt = ext
				break
			}
		}
	}

	// Serve the cached data if found
	if data != nil {
		println("[ServeHTTP] Cache HIT for ID:", idStr, "ext:", foundExt, "size:", len(data))
		w.Header().Set("Content-Type", getMimeType(foundExt))
		w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year in WebView
		_, _ = w.Write(data)
		return
	}

	println("[ServeHTTP] Cache MISS for ID:", idStr, "coverURL:", coverURL)

	// If not found in cache and we have a cover URL, download and cache it
	if coverURL != "" {
		// DownloadCover handles both full URLs and relative paths
		d, err := s.client.DownloadCover(coverURL)
		if err == nil {
			ext := urlExt
			if ext == "" {
				ext = ".jpg"
			}
			_ = os.MkdirAll(cacheDir, 0o755)
			cachePath := filepath.Join(cacheDir, idStr+ext)
			_ = os.WriteFile(cachePath, d, 0o644)

			println("[ServeHTTP] Downloaded and cached cover for ID:", idStr, "ext:", ext, "size:", len(d))
			w.Header().Set("Content-Type", getMimeType(ext))
			w.Header().Set("Cache-Control", "public, max-age=31536000")
			_, _ = w.Write(d)
			return
		} else {
			println("[ServeHTTP] Failed to download cover for ID:", idStr, "err:", err.Error())
		}
	}

	println("[ServeHTTP] Not found on cache and no/failed download, returning 404")
	http.NotFound(w, r)
}
