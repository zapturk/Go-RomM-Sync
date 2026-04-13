package retroarch

import (
	"go-romm-sync/types"
	"os"
	"path/filepath"
	"strings"
)

// LibraryProvider defines the dependency for resolving local ROM directories.
type LibraryProvider interface {
	GetRomDir(game *types.Game) string
}

// CoreResolver resolves the best libretro core(s) for a given game using a
// multi-strategy fallback chain.
type CoreResolver struct {
	library LibraryProvider
}

// NewCoreResolver creates a CoreResolver.
func NewCoreResolver(lib LibraryProvider) *CoreResolver {
	return &CoreResolver{library: lib}
}

// ResolveOptions carries the inputs needed to resolve cores for a game.
type ResolveOptions struct {
	PlatformSlug string // canonical slug, e.g. "snes"
	FullPath     string // server-side full path, e.g. "snes/game.sfc"
	LastUsed     string // previously saved core preference for this platform
}

// Resolve returns an ordered list of candidate cores for the given options.
// The first entry is the recommended default. Returns nil if nothing is found.
func (r *CoreResolver) Resolve(opts ResolveOptions) []string {
	var all []string

	// Strategy 1: platform slug direct lookup
	if opts.PlatformSlug != "" {
		if cores := GetCoresForPlatform(opts.PlatformSlug); len(cores) > 0 {
			all = append(all, cores...)
		}
	}

	// Strategy 2: platform slug derived from path segments
	if len(all) == 0 {
		fullPath := filepath.ToSlash(opts.FullPath)
		for _, part := range strings.Split(strings.TrimPrefix(fullPath, "/"), "/") {
			if cores := GetCoresForPlatform(part); len(cores) > 0 {
				all = append(all, cores...)
				break
			}
		}
	}

	// Strategy 3: extension from server-side filename (skip .zip — too ambiguous)
	if len(all) == 0 {
		ext := strings.ToLower(filepath.Ext(filepath.Base(opts.FullPath)))
		if ext != ".zip" {
			if cores := GetCoresForExt(ext); len(cores) > 0 {
				all = append(all, cores...)
			}
		}
	}

	// Strategy 4: local file scan (handles zips with real content inside)
	if len(all) == 0 && r.library != nil {
		if cores := r.scanLocalFiles(opts.FullPath); len(cores) > 0 {
			all = append(all, cores...)
		}
	}

	if len(all) == 0 {
		return nil
	}

	return PrioritizeCore(all, opts.LastUsed)
}

// scanLocalFiles scans the local ROM directory for a game and returns cores
// based on the actual files present (including peeking inside ZIPs).
func (r *CoreResolver) scanLocalFiles(fullPath string) []string {
	dir := r.library.GetRomDir(&types.Game{FullPath: fullPath})
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		itemPath := filepath.Join(dir, entry.Name())
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".zip" {
			if cores := GetCoresFromZip(itemPath); len(cores) > 0 {
				return cores
			}
		} else if cores := GetCoresForExt(ext); len(cores) > 0 {
			return cores
		}
	}
	return nil
}

// PrioritizeCore moves lastUsed to the front of the list without duplicating it.
// If lastUsed is not already in cores it is prepended (acts as a forced preference).
func PrioritizeCore(cores []string, lastUsed string) []string {
	if lastUsed == "" {
		return cores
	}
	result := []string{lastUsed}
	for _, c := range cores {
		if c != lastUsed {
			result = append(result, c)
		}
	}
	return result
}
