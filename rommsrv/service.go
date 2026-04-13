package rommsrv

import (
	"fmt"
	"go-romm-sync/retroarch"
	"go-romm-sync/romm"
	"go-romm-sync/types"
)

// ConfigProvider defines the configuration needed for RomM services.
type ConfigProvider interface {
	GetRomMHost() string
	GetUsername() string
	GetPassword() string
	GetClientToken() string
}

// Service handles interactions with the RomM server and manages local caches for assets.
type Service struct {
	config ConfigProvider
	client *romm.Client
}

// New creates a new RomM service.
func New(cfg ConfigProvider) *Service {
	host := cfg.GetRomMHost()
	client := romm.NewClient(host)
	client.Token = cfg.GetClientToken()
	return &Service{
		config: cfg,
		client: client,
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

// SetClientToken updates the active client's auth token.
func (s *Service) SetClientToken(token string) {
	s.client.Token = token
}

// ResetClient re-initialises the RomM client, clearing any in-memory session.
func (s *Service) ResetClient() {
	s.client = romm.NewClient(s.config.GetRomMHost())
}

// CreateClientToken creates a persistent client token via the RomM API.
func (s *Service) CreateClientToken(name string, scopes []string) (string, error) {
	return s.client.CreateClientToken(name, scopes)
}

// GetLibrary fetches a page of the game library from RomM, optionally filtered by platform and search query.
func (s *Service) GetLibrary(limit, offset, platformID int, search string) ([]types.Game, int, error) {
	return s.client.GetLibrary(limit, offset, platformID, search)
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

func (s *Service) GetFirmware(platformID uint) ([]types.Firmware, error) {
	return s.client.GetFirmware(platformID)
}

// GetServerSaves gets a list of server saves from RomM.
func (s *Service) GetServerSaves(id uint) ([]types.ServerSave, error) {
	return s.client.GetSaves(id)
}

// GetServerStates gets a list of server states from RomM.
func (s *Service) GetServerStates(id uint) ([]types.ServerState, error) {
	return s.client.GetStates(id)
}
