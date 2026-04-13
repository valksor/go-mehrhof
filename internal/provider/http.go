package provider

import (
	"net/http"
	"time"
)

// httpClient is a shared HTTP client with sensible timeouts for provider API calls.
// Using a shared client enables connection reuse and prevents resource exhaustion.
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	},
}
