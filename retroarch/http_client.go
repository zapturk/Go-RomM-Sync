package retroarch

import (
	"net/http"
	"time"
)

// httpTimeoutClient is a shared HTTP client with a 30-second timeout for
// all retroarch package downloads (BIOS, cores, PCSX2 resources).
var httpTimeoutClient = &http.Client{
	Timeout: 30 * time.Second,
}
