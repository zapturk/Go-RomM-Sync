package authsrv

import (
	"fmt"
	"os"

	"go-romm-sync/constants"
	"go-romm-sync/retroarch"
	"go-romm-sync/types"
)

// ConfigProvider defines the config access needed for auth.
type ConfigProvider interface {
	GetConfig() types.AppConfig
	Update(fn func(*types.AppConfig)) error
}

// RomMProvider defines the RomM interactions needed for auth.
type RomMProvider interface {
	Login() (string, error)
	GetPlatforms(limit, offset int) ([]types.Platform, int, error)
	CreateClientToken(name string, scopes []string) (string, error)
	SetClientToken(token string)
	ResetClient()
}

// UIProvider defines logging and event emission needed for auth.
type UIProvider interface {
	LogInfof(format string, args ...interface{})
	LogErrorf(format string, args ...interface{})
	EventsEmit(eventName string, args ...interface{})
}

// Service handles authentication and session management.
type Service struct {
	config ConfigProvider
	romm   RomMProvider
	ui     UIProvider
}

// New creates a new Auth service.
func New(cfg ConfigProvider, romm RomMProvider, ui UIProvider) *Service {
	return &Service{config: cfg, romm: romm, ui: ui}
}

// Login authenticates with RomM, attempting to upgrade to a persistent client token.
// Returns "persistent_token" if a client token is active, otherwise the session token.
func (s *Service) Login() (string, error) {
	// 1. Check for an existing persistent client token
	cfg := s.config.GetConfig()
	if cfg.ClientToken != "" && len(cfg.ClientToken) > 4 {
		s.romm.SetClientToken(cfg.ClientToken)

		// Verify it still works
		_, _, err := s.romm.GetPlatforms(1, 0)
		if err == nil {
			s.ui.LogInfof("Using persistent RomM Client Token for session")
			return "persistent_token", nil
		}
		s.ui.LogErrorf("Persistent Client Token failed validation: %v. Falling back to password login.", err)
	}

	// 2. Traditional login
	token, err := s.romm.Login()
	if err != nil {
		return "", err
	}

	// 3. Attempt upgrade to persistent client token
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "Go-RomM-Sync Client"
	}
	tokenName := fmt.Sprintf("Go-RomM-Sync (%s)", hostname)

	s.ui.LogInfof("Attempting to auto-upgrade to persistent RomM Client Token: %s", tokenName)
	clientToken, err := s.romm.CreateClientToken(tokenName, constants.RomMDefaultScopes)
	if err != nil {
		s.ui.LogErrorf("Failed to auto-upgrade to Client Token: %v. Continuing with current session.", err)
		return token, nil
	}

	// 4. Persist the new token
	if err := s.config.Update(func(c *types.AppConfig) {
		c.ClientToken = clientToken
	}); err != nil {
		s.ui.LogErrorf("Failed to save Client Token to config: %v. Continuing with current session.", err)
		return token, nil
	}

	tokenSnippet := "none"
	if len(clientToken) > 8 {
		tokenSnippet = clientToken[:4] + "..." + clientToken[len(clientToken)-4:]
	}
	s.ui.LogInfof("Successfully upgraded to persistent RomM Client Token: %s (%s)", tokenName, tokenSnippet)

	s.romm.SetClientToken(clientToken)
	s.ui.EventsEmit("config-updated", "client_token")

	return "persistent_token", nil
}

// Logout clears all credentials and resets the RomM client.
func (s *Service) Logout() error {
	if err := s.config.Update(func(c *types.AppConfig) {
		c.Username = ""
		c.Password = ""
		c.ClientToken = ""
		c.CheevosUsername = ""
		c.CheevosPassword = ""
	}); err != nil {
		return err
	}

	cfg := s.config.GetConfig()
	if cfg.RetroArchPath != "" {
		if err := retroarch.ClearCheevosToken(cfg.RetroArchPath); err != nil {
			s.ui.LogErrorf("Failed to clear RetroArch cheevos token: %v", err)
		}
	}

	s.romm.ResetClient()
	return nil
}
