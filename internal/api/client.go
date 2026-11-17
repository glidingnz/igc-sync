package api

import (
	"net/http"
	"time"
)

type userAgentTransport struct {
	userAgent string
	base      http.RoundTripper
}

func (t userAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("User-Agent", t.userAgent)
	return t.base.RoundTrip(r)
}

// NewClient identifies igc-sync by name, so a WAF in front of gliding.net.nz
// can allow it rather than challenging a generic Go-http-client.
func NewClient(version string) *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: userAgentTransport{
			userAgent: "igc-sync/" + version + " (+https://github.com/glidingnz/igc-sync)",
			base:      http.DefaultTransport,
		},
	}
}
