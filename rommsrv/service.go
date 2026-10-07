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
			if isPlatformSupported(&p) {
				if foundCount >= offset && len(supported) < limit {
					supported = append(supported, p)
				}
				foundCount++
			}
		}

		currentOffset += len(batch)

		// 2. Optimization: if we've filled our page OR reached an upper scan limit
		if (len(supported) >= limit && currentOffset >= totalOnServer) || currentOffset >= maxScan {
			// foundCount already tracks the total supported platforms we've scanned.
			break
		}

		if currentOffset >= totalOnServer {
			break
		}
	}

	return supported, foundCount, nil
}

func isPlatformSupported(p *types.Platform) bool {
	// Check if supported by RetroArch and has games
	return p.RomCount > 0 && (retroarch.IdentifyPlatform(p.Name) != "" || retroarch.IdentifyPlatform(p.Slug) != "")
}

// GetRom fetches a single ROM from RomM.
func (s *Service) GetRom(id uint) (types.Game, error) {
	return s.client.GetRom(id)
}

// GetPlatform fetches a single platform from RomM.
func (s *Service) GetPlatform(id uint) (types.Platform, error) {
	return s.client.GetPlatform(id)
}

func (s *Service) GetFirmware(platformID uint) ([]types.Firmware, error) {
	return s.client.GetFirmware(platformID)
}

// GetServerSaves gets a list of server saves from RomM.
func (s *Service) GetServerSaves(id uint) ([]types.ServerSave, error) {
	return s.client.GetSaves(id)
}

// GetServerSavesForSlot gets server saves for a specific slot from RomM.
func (s *Service) GetServerSavesForSlot(id uint, slot string) ([]types.ServerSave, error) {
	return s.client.GetSavesForSlot(id, slot)
}

// GetSaveSlots fetches available save slots for a ROM from RomM.
// It tries GetSaveSummary first, and falls back to grouping GetSaves by slot.
func (s *Service) GetSaveSlots(romID uint) ([]types.SaveSlot, error) {
	summary, err := s.client.GetSaveSummary(romID)
	if err == nil && summary != nil {
		var slots []types.SaveSlot
		for _, item := range summary.Slots {
			slotName := ""
			if item.Slot != nil {
				slotName = *item.Slot
			}
			slots = append(slots, types.SaveSlot{
				Slot:            slotName,
				Count:           item.Count,
				LatestUpdatedAt: item.Latest.UpdatedAt,
			})
		}
		return slots, nil
	}

	// Fallback: derive slots from GetSaves
	saves, err := s.client.GetSaves(romID)
	if err != nil {
		return nil, err
	}
	slotsMap := make(map[string]*types.SaveSlot)
	var order []string
	for i := range saves {
		name := saves[i].Slot
		if existing, ok := slotsMap[name]; ok {
			existing.Count++
			if saves[i].UpdatedAt > existing.LatestUpdatedAt {
				existing.LatestUpdatedAt = saves[i].UpdatedAt
			}
		} else {
			slot := &types.SaveSlot{
				Slot:            name,
				Count:           1,
				LatestUpdatedAt: saves[i].UpdatedAt,
			}
			slotsMap[name] = slot
			order = append(order, name)
		}
	}
	var slots []types.SaveSlot
	for _, name := range order {
		slots = append(slots, *slotsMap[name])
	}
	return slots, nil
}

// DeleteServerSaves deletes server saves on RomM by their IDs.
func (s *Service) DeleteServerSaves(saveIDs []uint) error {
	return s.client.DeleteServerSaves(saveIDs)
}

// GetServerStates gets a list of server states from RomM.
func (s *Service) GetServerStates(id uint) ([]types.ServerState, error) {
	return s.client.GetStates(id)
}

// DownloadCover downloads a cover image from RomM using the active client.
func (s *Service) DownloadCover(url string) ([]byte, error) {
	return s.client.DownloadCover(url)
}
