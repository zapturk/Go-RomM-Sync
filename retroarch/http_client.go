package retroarch

import (
	"net/http"
	"time"
)

// httpTimeoutClient is a shared HTTP client with a 30-second timeout for
// quick metadata/API requests (GitHub release info, PCSX2 GameIndex).
var httpTimeoutClient = &http.Client{
	Timeout: 30 * time.Second,
}

// httpDownloadClient is a shared HTTP client for large file downloads (BIOS, cores).
var httpDownloadClient = &http.Client{
	Timeout: 2 * time.Hour,
}

