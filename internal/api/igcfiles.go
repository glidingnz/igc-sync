package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// IgcFile represents a single IGC file entry from the API.
type IgcFile struct {
	ID           int    `json:"id"`
	Filename     string `json:"filename"`
	FlightDate   string `json:"flight_date"` // "YYYY-MM-DD"
	Rego         string `json:"rego"`
	FlightNumber int    `json:"flight_number"`
	SizeBytes    int64  `json:"size_bytes"`
	FileHash     string `json:"file_hash"` // SHA-256 hex digest
	URL          string `json:"url"`
}

type igcFilesResponse struct {
	Success     bool      `json:"success"`
	Data        []IgcFile `json:"data"`
	NextPageURL *string   `json:"next_page_url"`
}

// FetchIgcFiles retrieves all IGC files for the given event, following pagination.
// Relative URLs in the url field are resolved against baseURL.
func FetchIgcFiles(baseURL string, eventID int, client *http.Client) ([]IgcFile, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing base URL: %w", err)
	}

	var all []IgcFile
	pageURL := fmt.Sprintf("%s/api/v1/events/%d/igc-files", baseURL, eventID)

	for pageURL != "" {
		resp, err := client.Get(pageURL)
		if err != nil {
			return nil, fmt.Errorf("fetching igc-files page: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("igc-files API returned status %d", resp.StatusCode)
		}

		var page igcFilesResponse
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decoding igc-files response: %w", err)
		}
		resp.Body.Close()

		// Resolve relative URLs so Download always receives an absolute URL.
		for i := range page.Data {
			if page.Data[i].URL != "" {
				ref, err := url.Parse(page.Data[i].URL)
				if err == nil && !ref.IsAbs() {
					page.Data[i].URL = base.ResolveReference(ref).String()
				}
			}
		}

		all = append(all, page.Data...)

		if page.NextPageURL != nil && *page.NextPageURL != "" {
			pageURL = *page.NextPageURL
		} else {
			pageURL = ""
		}
	}

	return all, nil
}
